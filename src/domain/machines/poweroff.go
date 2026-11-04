package machines

import (
	"context"
	"time"

	"github.com/jterrazz/studio-station/src/machines/contract"
)

const (
	stoppedPoll   = 2 * time.Second
	stoppedBudget = 60 * time.Second
	hardStopWait  = 10 * time.Second
	natIPWait     = 10 * time.Second
)

const diskMissing contract.DiskState = ""

func (s *Service) engineState(ctx context.Context, name string) contract.DiskState {
	disks, err := s.engine.Disks(ctx)
	if err != nil {
		return diskMissing
	}
	for _, d := range disks {
		if d.VM == name {
			return d.State
		}
	}
	return diskMissing
}

// invariant: a graceful stop is the guest's own poweroff, so its station-intent
// unit runs and the stop reads as declared; the hard stop is only for a guest
// that ignored it past the budget.
func (s *Service) powerOff(ctx context.Context, name, user string) error {
	if s.engineState(ctx, name) != contract.DiskRunning {
		return nil
	}
	if addr, err := s.engine.Address(ctx, name, natIPWait); err == nil {
		// outside: ssh exits 255 when the poweroff cuts its own session, which is
		// the success path.
		if _, err := s.nat(ctx, addr.String(), user, "", 30*time.Second, "sudo", "systemctl", "--no-block", "poweroff"); err != nil {
			return err
		}
		for waited := time.Duration(0); waited < stoppedBudget; waited += stoppedPoll {
			if s.engineState(ctx, name) != contract.DiskRunning {
				return nil
			}
			if err := s.h.Clock.Sleep(ctx, stoppedPoll); err != nil {
				return err
			}
		}
	}
	return s.engine.Shutdown(ctx, name, hardStopWait)
}

func (s *Service) PowerOff(ctx context.Context, name string) error {
	return s.powerOff(ctx, name, s.VMUser())
}

func (s *Service) EngineRunning(ctx context.Context, name string) bool {
	return s.engineState(ctx, name) == contract.DiskRunning
}
