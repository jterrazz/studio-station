package machines

import (
	"context"
	"errors"
	"github.com/jterrazz/studio-station/src/machines/contract"

	"github.com/jterrazz/studio-station/src/catalog"
)

type NotAVMError struct{ Name, Verb string }

func (e *NotAVMError) Error() string { return "\"" + e.Name + "\" is a host, not a VM" }

type TemporaryStopError struct{ Name string }

func (e *TemporaryStopError) Error() string {
	return e.Name + " is temporary: stopping it would delete it"
}

type TemporaryStartError struct{ Name string }

func (e *TemporaryStartError) Error() string {
	return e.Name + " is temporary: it runs once, and a stopped one is stale"
}

type DeleteReport struct {
	Name     string
	Warnings []string
}

func (s *Service) VM(name, verb string) (VMRecord, error) {
	if err := s.CheckHost(); err != nil {
		return VMRecord{}, err
	}
	m, err := s.lookup(name)
	if err != nil {
		return VMRecord{}, err
	}
	if m.Kind == catalog.Host {
		return VMRecord{}, &NotAVMError{Name: name, Verb: verb}
	}
	records, err := s.records()
	if err != nil {
		return VMRecord{}, err
	}
	for _, r := range records {
		if r.Name == name {
			return r, nil
		}
	}
	return VMRecord{}, &UnknownMachineError{Name: name}
}

func (s *Service) CheckStart(name string) error {
	rec, err := s.VM(name, "start")
	if err == nil && rec.Lifetime == Temporary {
		return &TemporaryStartError{Name: name}
	}
	return err
}

func (s *Service) CheckStop(name string) error {
	rec, err := s.VM(name, "stop")
	if err == nil && rec.Lifetime == Temporary {
		return &TemporaryStopError{Name: name}
	}
	return err
}

func (s *Service) Start(ctx context.Context, name string) error {
	if err := s.CheckStart(name); err != nil {
		return err
	}
	if err := s.daemonUp(ctx); err != nil {
		return err
	}
	return s.daemon.Do(ctx, "start", name)
}

func (s *Service) Stop(ctx context.Context, name string) error {
	// invariant: a stopped lasting VM stays stopped until `start`, because its
	// record says so; with the daemon down the stop runs here, so a broken daemon
	// never holds a VM hostage.
	if err := s.CheckStop(name); err != nil {
		return err
	}
	if s.daemon != nil {
		err := s.daemon.Do(ctx, "stop", name)
		if !errors.Is(err, ErrDaemonDown) {
			return err
		}
	}
	return s.StopDirect(ctx, name)
}

func (s *Service) StopDirect(ctx context.Context, name string) error {
	rec, err := s.VM(name, "stop")
	if err != nil {
		return err
	}
	rec.Want = WantStopped
	if err := s.writeRecord(rec); err != nil {
		return err
	}
	if err := s.powerOff(ctx, name, s.VMUser()); err != nil {
		return err
	}
	s.retireAgent(ctx, name)
	return nil
}

func (s *Service) restartVM(ctx context.Context, name string, say func(phase, detail string)) (RestartReport, error) {
	rec, err := s.VM(name, "restart")
	if err != nil {
		return RestartReport{}, err
	}
	if rec.Lifetime == Temporary {
		return RestartReport{}, &TemporaryRestartError{Name: name}
	}
	if err := s.daemonUp(ctx); err != nil {
		return RestartReport{}, err
	}
	started := s.h.Clock.Now()
	say("restarting", "poweroff, then start")
	if err := s.daemon.Do(ctx, "restart", name); err != nil {
		return RestartReport{}, err
	}
	say("back", "the daemon runs it again")
	return RestartReport{Name: name, Waited: s.h.Clock.Now().Sub(started)}, nil
}

func (s *Service) Delete(ctx context.Context, name string) (DeleteReport, error) {
	if _, err := s.VM(name, "delete"); err != nil {
		return DeleteReport{}, err
	}
	if s.daemon != nil {
		err := s.daemon.Do(ctx, "delete", name)
		if err == nil {
			return DeleteReport{Name: name}, nil
		}
		if !errors.Is(err, ErrDaemonDown) {
			return DeleteReport{}, err
		}
	}
	return s.Remove(ctx, name)
}

func (s *Service) Remove(ctx context.Context, name string) (DeleteReport, error) {
	// invariant: Remove is the whole delete, run by the daemon or, when it is down,
	// by the CLI: the run stops, the disk, record, files and block go, and the VM's
	// leases and their serve entries are released.
	report := DeleteReport{Name: name}
	s.retireAgent(ctx, name)
	if s.engineState(ctx, name) == contract.DiskRunning {
		if err := s.engine.Shutdown(ctx, name, hardStopWait); err != nil {
			return report, err
		}
	}
	if s.engineState(ctx, name) != diskMissing {
		if err := s.engine.Delete(ctx, name); err != nil {
			return report, err
		}
	}
	if s.rel != nil {
		if err := s.rel.ReleaseVM(ctx, name); err != nil {
			report.Warnings = append(report.Warnings, err.Error())
		}
	}
	s.removeFiles(name, true)
	if err := s.editConfig(nil, false, []string{name}); err != nil {
		report.Warnings = append(report.Warnings, err.Error())
	}
	return report, nil
}
