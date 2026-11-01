package machines

import (
	"context"
	"errors"
	"io"
	"strconv"
	"time"

	"github.com/jterrazz/studio-station/src/system"
)

const pipeIPWait = 5 * time.Second

type NotHereError struct{ Name, Host string }

func (e *NotHereError) Error() string { return e.Name + " runs on " + e.Host + ", not here" }

type StoppedError struct {
	Name     string
	Lifetime Lifetime
}

func (e *StoppedError) Error() string {
	if e.Lifetime == Temporary {
		return e.Name + " is stale"
	}
	return e.Name + " is stopped"
}

type SSHDClosedError struct {
	Name, Addr string
	Cause      error
}

func (e *SSHDClosedError) Error() string { return e.Name + "'s sshd does not answer on " + e.Addr }

func (e *SSHDClosedError) Unwrap() error { return e.Cause }

func (s *Service) CheckPipe(ctx context.Context, name string) (VMRecord, error) {
	if err := s.CheckHost(); err != nil {
		return VMRecord{}, err
	}
	m, err := s.lookup(name)
	var unknown *UnknownMachineError
	if errors.As(err, &unknown) {
		if host, ok := s.hostOf(ctx, name); ok {
			return VMRecord{}, &NotHereError{Name: name, Host: host}
		}
	}
	if err != nil {
		return VMRecord{}, err
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
	// invariant: the VM's NAT ip is read at each pipe, never written into a block:
	// Tart's DHCP lease may change across runs; the pipe needs no daemon.
	rec, err := s.CheckPipe(ctx, name)
	if err != nil {
		return err
	}
	if s.engineState(ctx, name) != system.VMRunning {
		return &StoppedError{Name: name, Lifetime: rec.Lifetime}
	}
	ip, err := s.engine.IP(ctx, name, pipeIPWait)
	if err != nil {
		return &SSHDClosedError{Name: name, Addr: name, Cause: err}
	}
	port := s.o.GuestPort
	if port == 0 {
		port = 22
	}
	if err := s.h.Sockets.Pipe(ctx, ip, port, in, out); err != nil {
		return &SSHDClosedError{Name: name, Addr: ip + ":" + strconv.Itoa(port), Cause: err}
	}
	return nil
}
