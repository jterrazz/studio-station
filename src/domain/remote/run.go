package remote

import (
	"context"

	"github.com/jterrazz/studio-station/src/system"
)

const sshUnreachable = 255

type UnreachableError struct{ Machine string }

func (e *UnreachableError) Error() string { return "cannot reach \"" + e.Machine + "\"" }

type Service struct{ run system.Runner }

func New(run system.Runner) *Service { return &Service{run: run} }

func (s *Service) Run(ctx context.Context, machine string, interactive bool, words []string) (int, error) {
	code, err := s.run.Attach(ctx, system.Cmd{Name: "ssh", Args: Argv(machine, interactive, words)}, system.ToStdout)
	if err != nil {
		return 0, err
	}
	if code == sshUnreachable {
		return 0, &UnreachableError{Machine: machine}
	}
	return code, nil
}
