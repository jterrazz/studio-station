package machines

import "context"

func (s *Service) Stale(ctx context.Context) ([]VM, error) {
	records, err := s.records()
	if err != nil || len(records) == 0 {
		return nil, err
	}
	// hazard: a listing that fails reads every temporary VM as stale, so the
	// cleanable that deletes them asks the engine first and stops when it fails.
	if _, listErr := s.engine.List(ctx); listErr != nil {
		return nil, &NoEngineError{Cause: listErr}
	}
	vms, err := s.vms(ctx, "", nil)
	if err != nil {
		return nil, err
	}
	var stale []VM
	for _, vm := range vms {
		if vm.State == Stale {
			stale = append(stale, vm)
		}
	}
	return stale, nil
}
