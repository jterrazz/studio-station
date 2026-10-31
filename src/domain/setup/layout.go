package setup

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"path"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/domain/settings"
	"github.com/jterrazz/studio-station/src/system"
)

const (
	legacyDir      = ".studio"
	authKeyAccount = "tailnet-authkey"
	backgroundNote = "tailscaled moves in the background"
)

const (
	legacyDNS       = "dns"
	legacyTailscale = "tailscale"
)

// invariant: the files of the old ~/.studio that station wrote, by name: the
// layout step moves or removes exactly these and never anything else.
func tailscaledKept() []string { return []string{"tailscaled.state", "tailscaled.log"} }
func tailscaledDrop() []string {
	return []string{"tailscaled.pid", "tailscaled.sock", "caffeinate.pid"}
}
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

// invariant: config.json is removed only after the keychain write and the new
// settings.json both succeeded, so a rerun after any failure finds its source.
func (s *Service) migrateSettings(ctx context.Context) error {
	data, err := s.h.FS.ReadFile(s.v1Path())
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	converted, authKey, err := settings.ConvertV1(data, s.h.Env.User())
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
	if authKey != "" {
		if err := s.keychain.Write(ctx, authKeyAccount, authKey); err != nil {
			return err
		}
	}
	if !s.exists(s.settingsPath()) {
		if err := s.store.Save(converted); err != nil {
			return err
		}
	}
	return s.h.FS.Remove(s.v1Path())
}

func (s *Service) layout(ctx context.Context, migrated error) (Step, error) {
	if migrated != nil {
		return failed(stepLayout, migrated)
	}
	if !s.exists(s.legacyPath()) {
		return Step{Name: stepLayout, State: catalog.StateOK, Detail: "nothing to move"}, nil
	}
	moved, err := s.moveDNS()
	if err != nil {
		return failed(stepLayout, err)
	}
	background, tailMoved, err := s.moveTailscaled(ctx)
	if err != nil {
		return failed(stepLayout, err)
	}
	if background {
		return Step{Name: stepLayout, State: catalog.StateDegraded, Detail: backgroundNote, Log: s.logPath(), Changed: true}, nil
	}
	removed, leftover := s.tidy()
	step := Step{Name: stepLayout, State: catalog.StateOK, Detail: "nothing to move", Changed: moved || tailMoved || removed}
	if step.Changed {
		step.Detail = "moved to the XDG folders"
	}
	if leftover {
		step.Detail += " (~/.studio keeps files that are not station's)"
	}
	return step, nil
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

func (s *Service) moveTailscaled(ctx context.Context) (background, moved bool, err error) {
	var pending []string
	for _, name := range tailscaledKept() {
		if s.exists(s.legacyPath(legacyTailscale, name)) {
			pending = append(pending, name)
		}
	}
	if len(pending) == 0 {
		return false, false, nil
	}
	running, err := s.daemonRunning(ctx)
	if err != nil {
		return false, false, err
	}
	if running && !s.o.Detached {
		if err := s.detach(); err != nil {
			return false, false, err
		}
		return true, false, nil
	}
	if running {
		if err := s.stopDaemon(ctx); err != nil {
			return false, false, err
		}
	}
	for _, name := range pending {
		if err := s.move(s.legacyPath(legacyTailscale, name), path.Join(s.o.Paths.State, "tailscaled", name)); err != nil {
			return false, false, err
		}
	}
	if running {
		if s.daemon != nil {
			if err := s.daemon.Start(ctx); err != nil {
				return false, false, err
			}
		}
	}
	return false, true, nil
}

func (s *Service) legacyPID() (int, bool) {
	data, err := s.h.FS.ReadFile(s.legacyPath(legacyTailscale, "tailscaled.pid"))
	if err != nil {
		return 0, false
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	return pid, err == nil && pid > 0
}

// invariant: the daemon that runs now was started from the old folder and is
// known by its pid file there; the tailnet's own control answers for any other.
func (s *Service) daemonRunning(ctx context.Context) (bool, error) {
	if pid, ok := s.legacyPID(); ok && s.h.Run.Alive(pid) {
		return true, nil
	}
	if s.daemon == nil {
		return false, nil
	}
	return s.daemon.Running(ctx)
}

func (s *Service) stopDaemon(ctx context.Context) error {
	pid, ok := s.legacyPID()
	if !ok || !s.h.Run.Alive(pid) {
		if s.daemon == nil {
			return nil
		}
		return s.daemon.Stop(ctx)
	}
	if err := s.h.Run.Signal(pid, system.SigTerm); err != nil {
		return err
	}
	for range 20 {
		if !s.h.Run.Alive(pid) {
			return nil
		}
		if err := s.h.Clock.Sleep(ctx, 250*time.Millisecond); err != nil {
			return err
		}
	}
	return s.h.Run.Signal(pid, system.SigKill)
}

// hazard: a run over the tailnet loses its session while tailscaled restarts, so
// the stop-move-restart triple runs in a process of its own, logging to setup.log.
func (s *Service) detach() error {
	if err := s.h.FS.MkdirAll(path.Dir(s.logPath()), 0o700); err != nil {
		return err
	}
	_, err := s.h.Run.Start(system.Cmd{Name: s.o.Executable, Args: []string{"setup"}, Env: []string{"STATION_XDG_DETACHED=1"}}, s.logPath())
	return err
}

// invariant: a destination holding the same bytes is a move already done, one
// holding other bytes is refused with both files untouched, and the source goes
// only after the copy is read back identical (tailscaled.state IS the node's identity).
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
	for _, name := range tailscaledDrop() {
		removed = s.drop(s.legacyPath(legacyTailscale, name)) || removed
	}
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
	for _, rel := range [][]string{{legacyTailscale, tailscaledKept()[0]}, {legacyTailscale, tailscaledKept()[1]}, {"bin", legacyBin()[0]}, {"bin", legacyBin()[1]}} {
		if s.exists(s.legacyPath(rel...)) {
			return true
		}
	}
	entries, err := s.h.FS.ReadDir(s.legacyPath(legacyDNS))
	return err == nil && len(entries) > 0
}
