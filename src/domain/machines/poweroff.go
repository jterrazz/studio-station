package machines

import (
	"context"
	"time"

	"github.com/jterrazz/studio-station/src/system"
)

const (
	stoppedPoll   = 2 * time.Second
	stoppedBudget = 60 * time.Second
	hardStopWait  = 10 * time.Second
	natIPWait     = 10 * time.Second
)

func (s *Service) engineState(ctx context.Context, name string) system.VMState {
	infos, err := s.engine.List(ctx)
	if err != nil {
		return system.VMMissing
	}
	for _, i := range infos {
		if i.Name == name {
			return i.State
		}
	}
	return system.VMMissing
}

// invariant: a graceful stop is the guest's own poweroff, so its
// station-intent unit runs and the supervisor reads the stop as declared; the hard
// stop is only for a guest that ignored it past the budget.
func (s *Service) powerOff(ctx context.Context, name, user string) error {
	if s.engineState(ctx, name) != system.VMRunning {
		return nil
	}
	if ip, err := s.engine.IP(ctx, name, natIPWait); err == nil {
		// outside: ssh exits 255 when the poweroff cuts its own session, which is
		// the success path.
		if _, err := s.nat(ctx, ip, user, "", 30*time.Second, "sudo", "systemctl", "--no-block", "poweroff"); err != nil {
			return err
		}
		for waited := time.Duration(0); waited < stoppedBudget; waited += stoppedPoll {
			if s.engineState(ctx, name) != system.VMRunning {
				return nil
			}
			if err := s.h.Clock.Sleep(ctx, stoppedPoll); err != nil {
				return err
			}
		}
	}
	return s.engine.Stop(ctx, name, hardStopWait)
}
