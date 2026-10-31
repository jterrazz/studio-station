package config

import (
	"context"
	"errors"
	"io/fs"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/system"
)

const probeBudget = 5 * time.Second

func (s *Service) probe(ctx context.Context, name string, args ...string) (stdout string, ok bool) {
	out, err := s.h.Run.Output(ctx, system.Cmd{Name: name, Args: args, Timeout: probeBudget})
	if err != nil || out.ExitCode != 0 {
		return strings.TrimSpace(out.Stdout), false
	}
	return strings.TrimSpace(out.Stdout), true
}

func (s *Service) must(ctx context.Context, name string, args ...string) (string, error) {
	out, err := s.h.Run.Output(ctx, system.Cmd{Name: name, Args: args})
	if err != nil {
		return "", err
	}
	if out.ExitCode != 0 {
		return strings.TrimSpace(out.Stdout), failure(name, args, out)
	}
	return strings.TrimSpace(out.Stdout), nil
}

func (s *Service) mustWith(ctx context.Context, stdin, name string, args ...string) error {
	out, err := s.h.Run.Output(ctx, system.Cmd{Name: name, Args: args, Stdin: stdin})
	if err != nil {
		return err
	}
	if out.ExitCode != 0 {
		return failure(name, args, out)
	}
	return nil
}

func (s *Service) attach(ctx context.Context, stdin, name string, args ...string) error {
	code, err := s.h.Run.Attach(ctx, system.Cmd{Name: name, Args: args, Stdin: stdin}, system.ToStderr)
	if err != nil {
		return err
	}
	if code != 0 {
		return errors.New(name + " " + strings.Join(args, " ") + " exited " + strconv.Itoa(code))
	}
	return nil
}

func failure(name string, args []string, out system.Output) error {
	reason := firstLine(out.Stderr)
	if reason == "" {
		reason = firstLine(out.Stdout)
	}
	if reason == "" {
		reason = "exited " + strconv.Itoa(out.ExitCode)
	}
	return errors.New(name + " " + strings.Join(args, " ") + ": " + reason)
}

func (s *Service) exists(p string) bool {
	_, err := s.h.FS.Stat(p)
	return err == nil
}

func (s *Service) write(p string, data []byte, perm fs.FileMode) error {
	if err := s.h.FS.MkdirAll(path.Dir(p), 0o750); err != nil {
		return err
	}
	return s.h.FS.WriteFile(p, data, perm)
}

func (s *Service) requireDarwin() error {
	if s.h.Env.GOOS() != "darwin" {
		return errors.New("this item only runs on macOS (got " + s.h.Env.GOOS() + ")")
	}
	return nil
}

func okStep(label, detail string) Step {
	return Step{Label: label, State: catalog.StateOK, Detail: detail}
}

func skipStep(label, detail string) Step {
	return Step{Label: label, State: catalog.StateAbsent, Detail: detail}
}

func oneLineOrDash(s string) string {
	fields := strings.Fields(firstLine(s))
	if len(fields) == 0 {
		return "-"
	}
	return strings.Join(fields, " ")
}

// outside: a machine with no `security` binary keeps nothing, so a missing tool
// reads as "not kept" rather than as a broken keychain.
func (s *Service) kept(ctx context.Context, account string) (bool, error) {
	_, found, err := s.kc.Read(ctx, account)
	if errors.Is(err, system.ErrNotFound) {
		return false, nil
	}
	return found, err
}
