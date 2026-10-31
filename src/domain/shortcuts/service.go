package shortcuts

import (
	"context"
	"errors"
	"strings"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/system"
)

type Service struct{ h system.Host }

func New(h system.Host) *Service { return &Service{h: h} }

func (s *Service) Git(ctx context.Context, name string, args []string) error {
	var shortcut *catalog.GitShortcut
	for i := range catalog.GitShortcuts {
		if catalog.GitShortcuts[i].Name == name {
			shortcut = &catalog.GitShortcuts[i]
		}
	}
	if shortcut == nil {
		return &UnknownShortcutError{Name: name}
	}
	for _, step := range shortcut.Steps {
		argv := append([]string{}, step.Args...)
		if step.AppendArgs != "" {
			argv = append(argv, step.AppendArgs+strings.Join(args, " "))
		}
		code, err := s.h.Run.Attach(ctx, system.Cmd{Name: "git", Args: argv}, system.ToStdout)
		if errors.Is(err, system.ErrNotFound) {
			return &NotInstalledError{Tool: "git"}
		}
		if err != nil {
			return err
		}
		if code != 0 && !step.IgnoreFailure {
			return &GitStepError{Shortcut: name, Step: "git " + step.Args[0], ExitCode: code}
		}
	}
	return nil
}
