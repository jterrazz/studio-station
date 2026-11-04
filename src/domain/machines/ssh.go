package machines

import (
	"context"
	"errors"

	model "github.com/jterrazz/studio-station/src/machines"
)

// value: the ssh command that reaches a machine of All machines, as the
// owner's ssh blocks write it: a Mac as its own login, a VM, here or on
// another Mac, as the VM user.
func (s *Service) SSH(ctx context.Context, name string) (model.SSHTarget, error) {
	if err := s.CheckHost(); err != nil {
		return model.SSHTarget{}, err
	}
	cfg, err := s.prefs()
	if err != nil {
		return model.SSHTarget{}, err
	}
	m, err := s.lookup(name)
	var unknown *UnknownMachineError
	if errors.As(err, &unknown) {
		if _, ok := s.hostOf(ctx, name); ok {
			return sshTarget(s.vmUser(cfg), name), nil
		}
	}
	if err != nil {
		return model.SSHTarget{}, err
	}
	return sshTarget(m.User, m.Name), nil
}

func sshTarget(user, host string) model.SSHTarget {
	command := "ssh " + host
	if user != "" {
		command = "ssh " + user + "@" + host
	}
	return model.SSHTarget{User: user, Host: host, Command: command}
}
