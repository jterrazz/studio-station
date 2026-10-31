package config

import (
	"context"
	"strings"

	"github.com/jterrazz/studio-station/src/catalog"
)

func (s *Service) installSshd(ctx context.Context) (Change, error) {
	return s.remoteLogin(ctx, "on")
}

func (s *Service) uninstallSshd(ctx context.Context) (Change, error) {
	return s.remoteLogin(ctx, "off")
}

// invariant: a failing systemsetup is an error: the step never reads as done when
// Remote Login did not change.
func (s *Service) remoteLogin(ctx context.Context, state string) (Change, error) {
	if err := s.requireDarwin(); err != nil {
		return Change{}, err
	}
	change := Change{Before: s.sshdFacts(ctx)}
	if _, err := s.must(ctx, "systemsetup", "-setremotelogin", state); err != nil {
		return change, err
	}
	change.Steps = []Step{okStep("Remote Login", state)}
	change.After = s.sshdFacts(ctx)
	return change, nil
}

// outside: the check runs as the user, and `systemsetup -getremotelogin` then exits 0
// with its error on stdout; `launchctl print-disabled system` is readable without root
// and carries the sshd state systemsetup sets.
func (s *Service) checkSshd(ctx context.Context) (state catalog.State, detail string) {
	out, ok := s.probe(ctx, "launchctl", "print-disabled", "system")
	if !ok || !strings.Contains(out, `"com.openssh.sshd" => enabled`) {
		return catalog.StateAbsent, ""
	}
	return catalog.StateOK, "Remote Login on"
}

func (s *Service) sshdFacts(ctx context.Context) []Fact {
	out, ok := s.probe(ctx, "systemsetup", "-getremotelogin")
	if !ok {
		out = "unreadable"
	}
	return []Fact{{Key: "systemsetup", Value: oneLineOrDash(out)}}
}
