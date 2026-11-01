package config

import (
	"bytes"
	"context"
	"io/fs"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/system"
)

func (s *Service) touch(e catalog.Touch) handler {
	target := s.home(e.Path)
	return handler{
		check: func(context.Context) (catalog.State, string) {
			if s.exists(target) {
				return catalog.StateOK, e.Path
			}
			return catalog.StateAbsent, ""
		},
		install: func(context.Context, map[string]string) (Change, error) {
			if s.exists(target) {
				return Change{Steps: []Step{okStep(e.Path, "already there")}}, nil
			}
			if err := s.write(target, nil, 0o600); err != nil {
				return Change{}, err
			}
			return Change{Steps: []Step{okStep(e.Path, "created")}}, nil
		},
	}
}

func (s *Service) dotfiles(e catalog.Dotfiles) handler {
	return handler{
		check: func(context.Context) (catalog.State, string) {
			present, differ := 0, false
			for _, c := range e.Files {
				if s.exists(s.home(c.To)) {
					present++
					if c.Mode != 0 && !s.same(c) {
						differ = true
					}
				}
			}
			switch {
			case present == len(e.Files) && differ:
				return catalog.StateDegraded, "differs from station's: station config install"
			case present == len(e.Files):
				return catalog.StateOK, e.Files[0].To
			case present == 0:
				return catalog.StateAbsent, ""
			}
			return catalog.StateDegraded, "some files are missing"
		},
		install: func(ctx context.Context, _ map[string]string) (Change, error) {
			var change Change
			for _, c := range e.Files {
				data, err := fs.ReadFile(s.files, c.From)
				if err != nil {
					return change, err
				}
				mode := c.Mode
				if mode == 0 {
					mode = 0o600
				}
				if err := s.write(s.home(c.To), data, mode); err != nil {
					return change, err
				}
				if err := s.h.FS.Chmod(s.home(c.To), mode); err != nil {
					return change, err
				}
				change.Steps = append(change.Steps, okStep(c.To, "written"))
			}
			if len(e.Reload) > 0 {
				change.Steps = append(change.Steps, s.reload(ctx, e.Reload)...)
			}
			return change, nil
		},
	}
}

// outside: a reload that fails is no failure: tmux with no server running exits
// non-zero, and the new file is read at the next start.
func (s *Service) reload(ctx context.Context, argv []string) []Step {
	args := make([]string, len(argv)-1)
	for i, a := range argv[1:] {
		args[i] = s.home(a)
	}
	out, err := s.h.Run.Output(ctx, system.Cmd{Name: argv[0], Args: args, Timeout: probeBudget})
	if err != nil || out.ExitCode != 0 {
		return nil
	}
	return []Step{okStep(argv[0], "reloaded")}
}

func (s *Service) same(c catalog.Copy) bool {
	want, err := fs.ReadFile(s.files, c.From)
	if err != nil {
		return false
	}
	got, err := s.h.FS.ReadFile(s.home(c.To))
	return err == nil && bytes.Equal(got, want)
}
