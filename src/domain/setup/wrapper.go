package setup

import (
	"context"
	"errors"
	"io/fs"
	"path"
	"strings"

	"github.com/jterrazz/studio-station/src/catalog"
)

const stepTools = "tools"

type Installer interface {
	Missing(ctx context.Context) ([]string, error)
	Install(ctx context.Context, names []string) error
	Superseded(ctx context.Context) []string
	RemoveFormula(ctx context.Context, formula string) error
}

func (s *Service) tools(ctx context.Context, interactive bool) (Step, error) {
	step := Step{Name: stepTools, State: catalog.StateOK}
	var done []string
	if s.o.Installer != nil {
		missing, err := s.o.Installer.Missing(ctx)
		if err != nil {
			return failed(stepTools, err)
		}
		switch {
		case len(missing) == 0:
		case !interactive:
			step.State = catalog.StateAbsent
			done = append(done, "missing: "+strings.Join(missing, " ")+": run station setup on a terminal")
		default:
			if err := s.o.Installer.Install(ctx, missing); err != nil {
				return failed(stepTools, err)
			}
			done = append(done, "installed "+strings.Join(missing, " "))
		}
		for _, formula := range s.o.Installer.Superseded(ctx) {
			if err := s.o.Installer.RemoveFormula(ctx, formula); err != nil {
				return failed(stepTools, err)
			}
			done = append(done, formula+" formula uninstalled: the app replaces it")
		}
	}
	wrote, err := s.writeWrapper()
	if err != nil {
		return failed(stepTools, err)
	}
	if wrote {
		done = append(done, "tailscale wrapper written: "+s.o.Expand.Display(s.wrapperPath()))
	}
	step.Detail = strings.Join(done, ", ")
	step.Changed = len(done) > 0
	return step, nil
}

func (s *Service) toolsCheck() Step {
	if s.wrapperDue() {
		return Step{Name: stepTools, State: catalog.StateDegraded, Detail: "tailscale wrapper due: station setup"}
	}
	return Step{Name: stepTools, State: catalog.StateOK}
}

func (s *Service) wrapperPath() string { return path.Join(s.o.Paths.Bin, "tailscale") }

func wrapper(cli string) []byte { return []byte("#!/bin/sh\nexec " + cli + " \"$@\"\n") }

// invariant: the App Store app's CLI answers only when its bundle's executable
// runs directly (a symlink to it crashes), so PATH gets a two-line script, and
// only where nothing else answers: a symlink is replaced, the script's own
// bytes are adopted, another tailscale on PATH or a file station did not
// write is never touched.
func (s *Service) wrapperDue() bool {
	if s.o.AppStoreCLI == nil {
		return false
	}
	cli := s.o.AppStoreCLI()
	if cli == "" {
		return false
	}
	at := s.wrapperPath()
	if _, err := s.h.FS.Readlink(at); err == nil {
		return true
	}
	if _, err := s.h.FS.Stat(at); err == nil {
		return false
	}
	if found, ok := s.h.Run.LookPath("tailscale"); ok && found != at {
		return false
	}
	return true
}

func (s *Service) writeWrapper() (bool, error) {
	if !s.wrapperDue() {
		return false, nil
	}
	at := s.wrapperPath()
	if err := s.h.FS.Remove(at); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return false, err
	}
	if err := s.h.FS.MkdirAll(s.o.Paths.Bin, 0o755); err != nil {
		return false, err
	}
	if err := s.h.FS.WriteFile(at, wrapper(s.o.AppStoreCLI()), 0o755); err != nil {
		return false, err
	}
	return true, s.h.FS.Chmod(at, 0o755)
}
