package machines

import (
	"context"
	"errors"
	"io"
	"strconv"
	"time"

	model "github.com/jterrazz/studio-station/src/machines"
	"github.com/jterrazz/studio-station/src/machines/contract"
)

const pipeIPWait = 5 * time.Second

func (s *Service) CheckPipe(ctx context.Context, name string) (model.VMRecord, error) {
	if err := s.CheckHost(); err != nil {
		return model.VMRecord{}, err
	}
	m, err := s.lookup(name)
	var unknown *model.UnknownMachineError
	if errors.As(err, &unknown) {
		if host, ok := s.hostOf(ctx, name); ok {
			return model.VMRecord{}, &model.NotHereError{Name: name, Host: host}
		}
	}
	if err != nil {
		return model.VMRecord{}, err
	}
	return s.VM(m.Name, "pipe")
}

func (s *Service) hostOf(ctx context.Context, name string) (string, bool) {
	fleet, err := s.Fleet(ctx, false)
	if err != nil {
		return "", false
	}
	for _, h := range fleet.Hosts {
		for _, vm := range h.VMs {
			if vm.Name == name {
				return h.Name, true
			}
		}
	}
	return "", false
}

func (s *Service) Pipe(ctx context.Context, name string, in io.Reader, out io.Writer) error {
	// invariant: the model.VM's NAT ip is read at each pipe, never written into a block:
	// the hypervisor's DHCP lease may change across runs; the pipe needs no daemon.
	rec, err := s.CheckPipe(ctx, name)
	if err != nil {
		return err
	}
	if s.engineState(ctx, name) != contract.DiskRunning {
		return &model.StoppedError{Name: name, Lifetime: rec.Lifetime}
	}
	addr, err := s.engine.Address(ctx, name, pipeIPWait)
	if err != nil {
		return &model.SSHDClosedError{Name: name, Addr: name, Cause: err}
	}
	ip := addr.String()
	port := s.o.GuestPort
	if port == 0 {
		port = 22
	}
	if err := s.h.Sockets.Pipe(ctx, ip, port, in, out); err != nil {
		return &model.SSHDClosedError{Name: name, Addr: ip + ":" + strconv.Itoa(port), Cause: err}
	}
	return nil
}
