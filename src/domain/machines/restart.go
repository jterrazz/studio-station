package machines

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jterrazz/studio-station/src/system"
)

const (
	phaseStop   = "stop"
	phaseReturn = "return"

	pollEvery    = 2 * time.Second
	stopBudget   = 60 * time.Second
	returnBudget = 180 * time.Second
)

type RestartEvent struct{ Phase, Detail string }

type RestartReport struct {
	Name   string
	Waited time.Duration
}

func (s *Service) Restart(ctx context.Context, name string, progress func(RestartEvent)) (RestartReport, error) {
	if err := s.CheckHost(); err != nil {
		return RestartReport{}, err
	}
	m, err := s.RemoteHost(name, "restart")
	if err != nil {
		return RestartReport{}, err
	}
	started := s.h.Clock.Now()
	say := func(phase, detail string) {
		if progress != nil {
			progress(RestartEvent{Phase: phase, Detail: detail})
		}
	}

	say("restarting", "sudo fdesetup authrestart -delayminutes 0")
	out, err := s.h.Run.Output(ctx, system.Cmd{
		Name: "ssh", Args: concat(batchSSH(), []string{m.Name, "sudo fdesetup authrestart -delayminutes 0"}),
		Timeout: 30 * time.Second,
	})
	// outside: ssh exits 255 when the Mac drops the connection to reboot, which is
	// the success path; any other non-zero exit is the Mac refusing.
	if err != nil && !errors.Is(err, system.ErrTimeout) {
		return RestartReport{}, err
	}
	if err == nil && out.ExitCode != 0 && out.ExitCode != 255 {
		return RestartReport{}, &RestartRefusedError{Name: name, Detail: firstLine(out.Stderr + out.Stdout)}
	}

	if err := s.await(ctx, m.Name, false, stopBudget); err != nil {
		return RestartReport{}, wrap(err, name, phaseStop)
	}
	say("down", "the Mac is rebooting")
	if err := s.await(ctx, m.Name, true, returnBudget); err != nil {
		return RestartReport{}, wrap(err, name, phaseReturn)
	}
	waited := s.h.Clock.Now().Sub(started)
	say("back", "ssh answers again")
	return RestartReport{Name: name, Waited: waited}, nil
}

type budgetError struct{}

func (budgetError) Error() string { return "budget spent" }

func wrap(err error, name, phase string) error {
	if errors.Is(err, budgetError{}) {
		return &RestartTimeoutError{Name: name, Phase: phase}
	}
	return err
}

func (s *Service) await(ctx context.Context, name string, up bool, budget time.Duration) error {
	for waited := time.Duration(0); ; waited += pollEvery {
		if s.sshTrue(ctx, name) == up {
			return nil
		}
		if waited >= budget {
			return budgetError{}
		}
		if err := s.h.Clock.Sleep(ctx, pollEvery); err != nil {
			return err
		}
	}
}

func firstLine(text string) string {
	for _, line := range strings.Split(text, "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			return trimmed
		}
	}
	return "no detail"
}
