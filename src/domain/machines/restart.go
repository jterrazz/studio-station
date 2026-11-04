package machines

import (
	"context"
	"errors"
	model "github.com/jterrazz/studio-station/src/machines"
	"strings"
	"time"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/machines/contract"
)

const (
	phaseStop   = "stop"
	phaseReturn = "return"

	pollEvery    = 2 * time.Second
	stopBudget   = 60 * time.Second
	returnBudget = 180 * time.Second
)

type (
	RestartEvent  = model.RestartEvent
	RestartReport = model.RestartReport
)

func (s *Service) Restart(ctx context.Context, name string, progress func(RestartEvent)) (RestartReport, error) {
	if err := s.CheckHost(); err != nil {
		return RestartReport{}, err
	}
	m, err := s.Remote(name)
	if err != nil {
		return RestartReport{}, err
	}
	say := func(phase, detail string) {
		if progress != nil {
			progress(RestartEvent{Phase: phase, Detail: detail})
		}
	}
	if m.Kind != catalog.Host {
		return s.restartVM(ctx, name, say)
	}
	started := s.h.Clock.Now()

	say("restarting", "a planned restart that unlocks itself")
	var refused *contract.RestartRefusedError
	if err := s.o.Unlocker.PlannedRestart(ctx, m.Name); errors.As(err, &refused) {
		return RestartReport{}, &RestartRefusedError{Name: name, Detail: refused.Detail}
	} else if err != nil {
		return RestartReport{}, err
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
		if s.answers(ctx, name) == up {
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

func (s *Service) AwaitAnswer(ctx context.Context, mac string) bool {
	return s.await(ctx, mac, true, returnBudget) == nil
}

func (s *Service) Answers(ctx context.Context, mac string) bool { return s.answers(ctx, mac) }

func (s *Service) answers(ctx context.Context, mac string) bool {
	if s.o.Answers != nil {
		return s.o.Answers(ctx, mac)
	}
	return s.sshTrue(ctx, mac)
}

func firstLine(text string) string {
	for _, line := range strings.Split(text, "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			return trimmed
		}
	}
	return "no detail"
}
