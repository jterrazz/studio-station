package setup

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"path"
	"reflect"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/domain/settings"
	"github.com/jterrazz/studio-station/src/system"
)

const legacyDir = ".studio"

const (
	legacyDNS       = "dns"
	legacyTailscale = "tailscale"
)

// invariant: the files of the old ~/.studio that station wrote, by name: the
// layout step moves or removes exactly these and never anything else.
func legacyBin() []string { return []string{"station", "workstation"} }

func (s *Service) legacyPath(parts ...string) string {
	return path.Join(append([]string{s.h.Env.Home(), legacyDir}, parts...)...)
}

func (s *Service) v1Path() string       { return s.legacyPath("config.json") }
func (s *Service) settingsPath() string { return path.Join(s.o.Paths.Config, "settings.json") }
func (s *Service) logPath() string      { return path.Join(s.o.Paths.State, "logs", "setup.log") }

func (s *Service) exists(p string) bool {
	_, err := s.h.FS.Stat(p)
	return err == nil
}

// invariant: config.json is removed only after the new settings.json is
// written, so a rerun after any failure finds its source.
func (s *Service) migrateSettings() error {
	data, err := s.h.FS.ReadFile(s.v1Path())
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	converted, err := settings.ConvertV1(data, s.h.Env.User())
	if err != nil {
		return err
	}
	if s.exists(s.settingsPath()) {
		existing, loadErr := s.store.Load()
		if loadErr != nil {
			return loadErr
		}
		if !reflect.DeepEqual(existing, converted) {
			return &DifferentContentError{Path: s.settingsPath()}
		}
	}
	if !s.exists(s.settingsPath()) {
		if err := s.store.Save(converted); err != nil {
			return err
		}
	}
	return s.h.FS.Remove(s.v1Path())
}

func (s *Service) layout(ctx context.Context, migrated error) ([]Step, error) {
	if migrated != nil {
		step, err := failed(stepLayout, migrated)
		return []Step{step}, err
	}
	retired, background, retireErr := s.retire(ctx)
	if retireErr != nil {
		step, err := failed(stepLayout, retireErr)
		return []Step{step}, err
	}
	if background {
		return retired, nil
	}
	if !s.exists(s.legacyPath()) {
		if len(retired) > 0 {
			return retired, nil
		}
		return []Step{{Name: stepLayout, State: catalog.StateOK, Detail: "nothing to retire"}}, nil
	}
	moved, moveErr := s.moveDNS()
	if moveErr != nil {
		step, err := failed(stepLayout, moveErr)
		return []Step{step}, err
	}
	removed, leftover := s.tidy()
	step := Step{Name: stepLayout, State: catalog.StateOK, Detail: "nothing to move", Changed: moved || removed}
	if step.Changed {
		step.Detail = "moved to the XDG folders"
	}
	if leftover {
		step.Detail += " (~/.studio keeps files that are not station's)"
	}
	return append([]Step{step}, retired...), nil
}

func (s *Service) moveDNS() (bool, error) {
	entries, err := s.h.FS.ReadDir(s.legacyPath(legacyDNS))
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	moved := false
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if err := s.move(s.legacyPath(legacyDNS, e.Name()), path.Join(s.o.Paths.State, "dns", e.Name())); err != nil {
			return moved, err
		}
		moved = true
	}
	return moved, nil
}

// hazard: a run over the tailnet loses its session while the retired node stops, so
// the retirement runs in a process of its own, logging to setup.log.
func (s *Service) detach() error {
	if err := s.h.FS.MkdirAll(path.Dir(s.logPath()), 0o700); err != nil {
		return err
	}
	_, err := s.h.Run.Start(system.Cmd{Name: s.o.Executable, Args: []string{"setup"}, Env: []string{"STATION_XDG_DETACHED=1"}}, s.logPath())
	return err
}

// invariant: a destination holding the same bytes is a move already done, one
// holding other bytes is refused with both files untouched, and the source goes
// only after the copy is read back identical (a node state file IS the node's identity).
func (s *Service) move(src, dst string) error {
	data, err := s.h.FS.ReadFile(src)
	if err != nil {
		return err
	}
	if existing, readErr := s.h.FS.ReadFile(dst); readErr == nil {
		if !bytes.Equal(existing, data) {
			return &DifferentContentError{Path: dst}
		}
		return s.h.FS.Remove(src)
	}
	if copyErr := s.copyVerified(src, dst, data); copyErr != nil {
		return copyErr
	}
	return s.h.FS.Remove(src)
}

func (s *Service) copyVerified(src, dst string, data []byte) error {
	perm := fs.FileMode(0o600)
	if info, statErr := s.h.FS.Stat(src); statErr == nil {
		perm = info.Mode().Perm()
	}
	if err := s.h.FS.MkdirAll(path.Dir(dst), 0o700); err != nil {
		return err
	}
	if err := s.h.FS.WriteFile(dst, data, perm); err != nil {
		return err
	}
	copied, err := s.h.FS.ReadFile(dst)
	if err != nil {
		return err
	}
	if !bytes.Equal(copied, data) {
		return &DifferentContentError{Path: dst}
	}
	return nil
}

func (s *Service) tidy() (removed, leftover bool) {
	for _, name := range legacyBin() {
		removed = s.drop(s.legacyPath("bin", name)) || removed
	}
	for _, dir := range []string{legacyTailscale, legacyDNS, "bin"} {
		removed = s.drop(s.legacyPath(dir)) || removed
	}
	removed = s.drop(s.legacyPath()) || removed
	return removed, s.exists(s.legacyPath())
}

func (s *Service) drop(p string) bool {
	return s.h.FS.Remove(p) == nil
}

func (s *Service) holdsStationFiles() bool {
	if s.exists(s.v1Path()) {
		return true
	}
	for _, rel := range [][]string{legacyNodeState(), {"bin", legacyBin()[0]}, {"bin", legacyBin()[1]}} {
		if s.exists(s.legacyPath(rel...)) {
			return true
		}
	}
	entries, err := s.h.FS.ReadDir(s.legacyPath(legacyDNS))
	return err == nil && len(entries) > 0
}
