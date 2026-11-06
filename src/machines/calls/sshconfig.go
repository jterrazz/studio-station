package calls

import (
	"context"
	"errors"

	model "github.com/jterrazz/studio-station/src/machines"

	"github.com/jterrazz/studio-station/src/machines/contract"
)

func (s *Service) SSHBlocks(ctx context.Context, f model.AllMachines) error {
	// hazard: the blocks of machines the fleet no longer holds are pruned only when
	// every host answered; an unreachable host's VMs are unknown, not gone.
	cfg, err := s.prefs()
	if err != nil {
		return err
	}
	var blocks []contract.MachineBlock
	complete := true
	key := cfg.Hosts[cfg.Here].Key
	for _, host := range f.Macs {
		complete = complete && host.Reachable
		if !host.ThisMac {
			h := cfg.Hosts[host.Name]
			blocks = append(blocks, contract.MachineBlock{Name: host.Name, User: h.User, Key: h.Key})
		}
		for _, vm := range host.VMs {
			blocks = append(blocks, contract.MachineBlock{Name: vm.Name, User: s.vmUser(cfg), Key: key, Behind: host.Name, ThisMac: host.ThisMac})
		}
	}
	write := s.o.SSHConfig.Put
	if complete {
		write = s.o.SSHConfig.Replace
	}
	return configFailure(write(ctx, blocks), false)
}

func configFailure(err error, rolledBack bool) error {
	if err == nil {
		return nil
	}
	var refused *contract.SSHConfigError
	if errors.As(err, &refused) {
		return &model.SSHConfigError{Path: refused.Path, Cause: refused.Cause, RolledBack: rolledBack}
	}
	return &model.SSHConfigError{Cause: err, RolledBack: rolledBack}
}
