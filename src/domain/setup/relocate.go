package setup

import (
	"context"
	"errors"
	"io/fs"
	"path"
	"slices"
	"strings"
	"time"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/domain/storage"
	"github.com/jterrazz/studio-station/src/platform/local"
)

const (
	stepRelocate = "relocate"
	busyBudget   = 2 * time.Second
)

type Configurer interface {
	Configure(ctx context.Context, item string, interactive bool) error
}

type BusyError struct{ Process string }

func (e *BusyError) Error() string { return e.Process + " is running: its files cannot move under it" }

type DestinationExistsError struct{ Path string }

func (e *DestinationExistsError) Error() string { return e.Path + " already exists" }

type CrossDeviceError struct{ From, To string }

func (e *CrossDeviceError) Error() string {
	return e.From + " and " + e.To + " are on different volumes: station moves, never copies"
}

func fileName(f catalog.ToolFile) string {
	if f == catalog.FileGoEnv {
		return "go env"
	}
	return string(f)
}

func keyLabel(file catalog.ToolFile, key string) string {
	switch file {
	case catalog.FileNpmrc:
		return "npm " + key
	case catalog.FileMise:
		return "mise " + strings.TrimPrefix(key, "settings.")
	case catalog.FileGoEnv, catalog.FileBunfig:
	}
	return key
}

func (s *Service) placedTools() []catalog.Tool {
	var out []catalog.Tool
	for _, t := range s.o.Tools {
		if slices.Contains(t.Places(), s.o.Kind) {
			out = append(out, t)
		}
	}
	return out
}

func (s *Service) relocate(ctx context.Context, interactive bool) (Step, error) {
	written, notes, err := s.writeToolFiles()
	if err != nil {
		return failed(stepRelocate, err)
	}
	var moved []string
	if s.onMac() {
		var moveNotes []string
		moved, moveNotes, err = s.moveStrays(ctx, interactive)
		notes = append(notes, moveNotes...)
		if err != nil {
			return failed(stepRelocate, err)
		}
	}
	switch {
	case len(notes) > 0:
		return Step{Name: stepRelocate, State: catalog.StateDegraded, Detail: strings.Join(notes, "; "), Changed: len(written)+len(moved) > 0}, nil
	case len(written)+len(moved) == 0:
		return Step{Name: stepRelocate, State: catalog.StateOK, Detail: "nothing to do"}, nil
	}
	parts := slices.Clone(written)
	if len(moved) > 0 {
		parts = append(parts, "moved "+strings.Join(moved, ", "))
	}
	return Step{Name: stepRelocate, State: catalog.StateOK, Detail: strings.Join(parts, ", "), Changed: true}, nil
}

func (s *Service) writeToolFiles() (written, notes []string, err error) {
	byFile := map[catalog.ToolFile][]catalog.Relocation{}
	var order []catalog.ToolFile
	for _, t := range s.placedTools() {
		for _, r := range t.Storage.Relocate {
			if r.Pending != "" {
				continue
			}
			if _, seen := byFile[r.File]; !seen {
				order = append(order, r.File)
			}
			byFile[r.File] = append(byFile[r.File], r)
		}
	}
	for _, file := range order {
		target := s.o.Files[file]
		if target == "" {
			notes = append(notes, fileName(file)+" is off (GOENV=off): station writes nothing")
			continue
		}
		current, readErr := s.h.FS.ReadFile(target)
		if readErr != nil && !errors.Is(readErr, fs.ErrNotExist) {
			return written, notes, readErr
		}
		text := string(current)
		changed := false
		if file == catalog.FileBunfig {
			values := map[string]string{}
			for _, r := range byFile[file] {
				values[r.Key] = s.o.Expand.Expand(r.Value)
			}
			updated, did, conflict := UpsertBunfig(text, values["install.globalDir"], values["install.globalBinDir"], values["install.cache.dir"])
			if conflict != "" {
				notes = append(notes, conflict)
			}
			text, changed = updated, did
		}
		for _, r := range byFile[file] {
			if file == catalog.FileBunfig {
				continue
			}
			value := s.o.Expand.Expand(r.Value)
			upsert := UpsertEnvLine
			if file == catalog.FileMise {
				upsert = UpsertTableKey
			}
			updated, did, hand := upsert(text, r.Key, value)
			if hand != "" {
				notes = append(notes, keyLabel(file, r.Key)+" set by hand to "+hand+"; station leaves it")
				continue
			}
			text, changed = updated, changed || did
		}
		if !changed {
			continue
		}
		if err := s.h.FS.MkdirAll(path.Dir(target), 0o755); err != nil {
			return written, notes, err
		}
		perm := fs.FileMode(0o644)
		if info, statErr := s.h.FS.Stat(target); statErr == nil {
			perm = info.Mode().Perm()
		}
		if err := s.h.FS.WriteFile(target, []byte(text), perm); err != nil {
			return written, notes, err
		}
		written = append(written, fileName(file))
	}
	return written, notes, nil
}

func UpsertEnvLine(text, key, value string) (updated string, changed bool, handSet string) {
	if current, ok := storage.LineValue(text, key); ok {
		if current == value {
			return text, false, ""
		}
		return text, false, current
	}
	if text != "" && !strings.HasSuffix(text, "\n") {
		text += "\n"
	}
	return text + key + "=" + value + "\n", true, ""
}

func (s *Service) moveStrays(ctx context.Context, interactive bool) (moved, notes []string, err error) {
	for _, t := range s.placedTools() {
		for _, st := range t.Storage.Strays {
			if !s.pending(st) {
				continue
			}
			if st.Busy != "" && s.busy(ctx, st.Busy) {
				return moved, notes, &BusyError{Process: st.Busy}
			}
			did, conflicts, moveErr := s.moveStray(st)
			notes = append(notes, conflicts...)
			if moveErr != nil {
				return moved, notes, moveErr
			}
			for _, dir := range st.Prune {
				_ = s.h.FS.Remove(s.o.Expand.Expand(dir))
			}
			if did {
				moved = append(moved, st.Path)
			}
			for _, item := range st.Then {
				if s.o.Configurer == nil {
					continue
				}
				if err := s.o.Configurer.Configure(ctx, item, interactive); err != nil {
					return moved, notes, err
				}
			}
		}
	}
	return moved, notes, nil
}

func (s *Service) pending(st catalog.Stray) bool {
	for _, m := range st.Move {
		if matches, _ := s.h.FS.Glob(s.o.Expand.Expand(m.From)); len(matches) > 0 {
			return true
		}
	}
	return false
}

func (s *Service) busy(ctx context.Context, process string) bool {
	out, err := s.h.Run.Output(ctx, local.Cmd{Name: "pgrep", Args: []string{"-x", process}, Timeout: busyBudget})
	return err == nil && out.ExitCode == 0
}

// invariant: every move is one rename(2), never a copy: an entry glob moves each
// match into the destination folder and keeps the owner's file where one of the same
// name is already there; a whole folder refuses a destination that holds anything.
func (s *Service) moveStray(st catalog.Stray) (did bool, conflicts []string, err error) {
	for _, m := range st.Move {
		from, to := s.o.Expand.Expand(m.From), s.o.Expand.Expand(m.To)
		entries := strings.HasSuffix(m.From, "/*")
		matches, _ := s.h.FS.Glob(from)
		for _, src := range matches {
			dst := to
			if entries {
				dst = path.Join(to, path.Base(src))
			}
			if !s.empty(dst) {
				if entries {
					conflicts = append(conflicts, s.o.Expand.Display(src)+" already in "+s.o.Expand.Display(to))
					continue
				}
				return did, conflicts, &DestinationExistsError{Path: s.o.Expand.Display(dst)}
			}
			if err := s.rename(src, dst); err != nil {
				return did, conflicts, err
			}
			did = true
		}
	}
	return did, conflicts, nil
}

func (s *Service) empty(p string) bool {
	info, err := s.h.FS.Stat(p)
	if err != nil {
		return true
	}
	if !info.IsDir() {
		return false
	}
	entries, err := s.h.FS.ReadDir(p)
	return err == nil && len(entries) == 0
}

func (s *Service) rename(from, to string) error {
	if err := s.h.FS.MkdirAll(path.Dir(to), 0o755); err != nil {
		return err
	}
	if s.exists(to) {
		if err := s.h.FS.Remove(to); err != nil {
			return err
		}
	}
	err := s.h.FS.Rename(from, to)
	if errors.Is(err, local.ErrCrossDevice) {
		return &CrossDeviceError{From: s.o.Expand.Display(from), To: s.o.Expand.Display(to)}
	}
	return err
}
