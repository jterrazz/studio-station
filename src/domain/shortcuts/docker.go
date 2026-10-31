package shortcuts

import (
	"context"
	"errors"
	"strings"

	"github.com/jterrazz/studio-station/src/system"
)

type Outcome struct {
	Shortcut string
	Removals []Removal
}

type Removal struct {
	What  string
	Count int
}

func (s *Service) Docker(ctx context.Context, name string) (Outcome, error) {
	var steps []func(context.Context) (Removal, error)
	switch name {
	case "rm":
		steps = append(steps, s.rm)
	case "rmi":
		steps = append(steps, s.rmi)
	case "reset":
		steps = append(steps, s.rm, s.rmi)
	default:
		return Outcome{}, &UnknownShortcutError{Name: name}
	}
	outcome := Outcome{Shortcut: name}
	for _, step := range steps {
		removal, err := step(ctx)
		if err != nil {
			return outcome, err
		}
		outcome.Removals = append(outcome.Removals, removal)
	}
	return outcome, nil
}

func (s *Service) rm(ctx context.Context) (Removal, error) {
	return s.removeAll(ctx, "containers", []string{"ps", "-aq"}, []string{"rm", "-vf"})
}

func (s *Service) rmi(ctx context.Context) (Removal, error) {
	return s.removeAll(ctx, "images", []string{"images", "-aq"}, []string{"rmi", "-f"})
}

func (s *Service) removeAll(ctx context.Context, what string, list, remove []string) (Removal, error) {
	out, err := s.docker(ctx, list)
	if err != nil {
		return Removal{}, err
	}
	ids := strings.Fields(out)
	if len(ids) == 0 {
		return Removal{What: what}, nil
	}
	if _, err := s.docker(ctx, append(append([]string{}, remove...), ids...)); err != nil {
		return Removal{}, err
	}
	return Removal{What: what, Count: len(ids)}, nil
}

func (s *Service) docker(ctx context.Context, args []string) (string, error) {
	out, err := s.h.Run.Output(ctx, system.Cmd{Name: "docker", Args: args})
	if errors.Is(err, system.ErrNotFound) {
		return "", &NotInstalledError{Tool: "docker"}
	}
	if err != nil {
		return "", err
	}
	if out.ExitCode != 0 {
		return "", &DockerStepError{Step: "docker " + args[0], Detail: firstLine(out.Stderr)}
	}
	return out.Stdout, nil
}

func firstLine(s string) string {
	line, _, _ := strings.Cut(strings.TrimSpace(s), "\n")
	return line
}
