package machines

import (
	"context"
	"errors"

	"github.com/jterrazz/studio-station/src/core/store"
	model "github.com/jterrazz/studio-station/src/machines"
	"github.com/jterrazz/studio-station/src/machines/contract"

	"github.com/jterrazz/studio-station/src/catalog"
)

func recordOf(r model.Record) VMRecord {
	return VMRecord{
		Name: r.Name, Image: r.Image, Version: r.Version, Lifetime: r.Lifetime, Want: r.Want, Network: r.Network, Check: r.Check, Env: r.Env,
		Resources: Resources{CPUs: r.CPUs, MemoryBytes: r.MemoryBytes, DiskBytes: r.DiskBytes},
		CreatedAt: r.CreatedAt,
	}
}

func storedOf(r VMRecord) model.Record {
	return model.Record{
		Name: r.Name, Image: r.Image, Version: r.Version, Lifetime: r.Lifetime, Want: r.Want, Network: r.Network, Check: r.Check, Env: r.Env,
		CPUs: r.Resources.CPUs, MemoryBytes: r.Resources.MemoryBytes, DiskBytes: r.Resources.DiskBytes, CreatedAt: r.CreatedAt,
	}
}

func (s *Service) records() ([]VMRecord, error) {
	stored, err := s.recs.List(context.Background())
	if err != nil {
		var unreadable *store.UnreadableError
		if errors.As(err, &unreadable) {
			return nil, &RecordError{Path: unreadable.Path, Cause: unreadable.Cause}
		}
		return nil, err
	}
	out := make([]VMRecord, 0, len(stored))
	for _, r := range stored {
		out = append(out, recordOf(r))
	}
	return out, nil
}

func kindOfImage(image string) catalog.MachineKind {
	for _, i := range catalog.Images {
		if i.Name == image {
			return i.Kind
		}
	}
	return catalog.Linux
}

func (s *Service) Records() ([]VMRecord, error) { return s.records() }

func (s *Service) vms(ctx context.Context, host string, live map[string]DaemonVM) ([]VM, error) {
	// invariant: the daemon's view wins; without it, no engine or a listing that
	// fails means nothing is running: every workspace reads stopped.
	records, err := s.records()
	if err != nil || len(records) == 0 {
		return nil, err
	}
	engine := map[string]contract.Disk{}
	if s.engine != nil {
		if disks, listErr := s.engine.Disks(ctx); listErr == nil {
			for _, d := range disks {
				engine[d.VM] = d
			}
		}
	}
	out := make([]VM, 0, len(records))
	for _, r := range records {
		info := engine[r.Name]
		vm := VM{
			Name: r.Name, Image: r.Image, Version: r.Version, Host: host, Kind: kindOfImage(r.Image),
			Lifetime: r.Lifetime, State: stateOf(info), Resources: r.Resources, SizeGB: int(info.Bytes / 1e9),
			Network: networkOf(r), Guest: "unknown", Want: r.Want, Check: r.Check, CreatedAt: r.CreatedAt,
		}
		if d, ok := live[r.Name]; ok && live != nil {
			if d.State != "" {
				vm.State = VMState(d.State)
			}
			vm.Guest, vm.IP, vm.Detail = d.Guest, d.IP, d.Detail
			vm.SSH, vm.Restarts, vm.CheckStatus = d.SSH, d.Restarts, d.Check
		}
		out = append(out, vm)
	}
	return out, nil
}

func networkOf(r VMRecord) string {
	if r.Network == "" {
		return NetworkNAT
	}
	return r.Network
}

func stateOf(d contract.Disk) VMState {
	switch d.State {
	case contract.DiskRunning:
		return Running
	case contract.DiskSuspended:
		return Suspended
	case contract.DiskStopped, diskMissing:
	}
	return Stopped
}
