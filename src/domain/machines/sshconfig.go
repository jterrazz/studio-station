package machines

import (
	"context"
	"errors"

	"github.com/jterrazz/studio-station/src/machines/contract"
)

func (s *Service) SSHBlocks(ctx context.Context, f Fleet) error {
	// hazard: the blocks of machines the fleet no longer holds are pruned only when
	// every host answered; an unreachable host's VMs are unknown, not gone.
	cfg, err := s.prefs()
	if err != nil {
		return err
	}
	var blocks []contract.HostBlock
	complete := true
	key := cfg.Hosts[cfg.Here].Key
	for _, host := range f.Hosts {
		complete = complete && host.Reachable
		if !host.Here {
			h := cfg.Hosts[host.Name]
			blocks = append(blocks, contract.HostBlock{Name: host.Name, User: h.User, Key: h.Key})
		}
		for _, vm := range host.VMs {
			blocks = append(blocks, contract.HostBlock{Name: vm.Name, User: s.vmUser(cfg), Key: key, Behind: host.Name, Here: host.Here})
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
		return &SSHConfigError{Path: refused.Path, Cause: refused.Cause, RolledBack: rolledBack}
	}
	return &SSHConfigError{Cause: err, RolledBack: rolledBack}
}
