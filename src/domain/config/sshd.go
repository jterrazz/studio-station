package config

import (
	"context"

	"github.com/jterrazz/studio-station/src/background/contract"
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
// with its error on stdout; the system domain's overrides are readable without root
// and carry the sshd state systemsetup sets.
func (s *Service) checkSshd(ctx context.Context) (state catalog.State, detail string) {
	if !s.o.Jobs.Overrides(ctx, contract.Domain{System: true})["com.openssh.sshd"] {
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
