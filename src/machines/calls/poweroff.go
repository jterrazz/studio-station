package calls

import (
	"context"
	"time"

	"github.com/jterrazz/studio-station/src/machines/contract"
)

const (
	stoppedBudget = 60 * time.Second
	hardStopWait  = 10 * time.Second
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

// invariant: a stop the engine cannot ask of its guest is the hypervisor's
// graceful shutdown: the guest powers itself off within the budget, its
// station-intent unit running as for its own poweroff, and only then is it cut.
func (s *Service) powerOff(ctx context.Context, name string) error {
	if s.engineState(ctx, name) != contract.DiskRunning {
		return nil
	}
	return s.engine.Shutdown(ctx, name, stoppedBudget)
}

func (s *Service) PowerOff(ctx context.Context, name string) error {
	return s.powerOff(ctx, name)
}

func (s *Service) EngineRunning(ctx context.Context, name string) bool {
	return s.engineState(ctx, name) == contract.DiskRunning
}
