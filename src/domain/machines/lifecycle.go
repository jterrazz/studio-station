package machines

import (
	"context"
	model "github.com/jterrazz/studio-station/src/machines"
	"github.com/jterrazz/studio-station/src/machines/contract"

	"github.com/jterrazz/studio-station/src/catalog"
)

type NotAVMError struct{ Name, Verb string }

func (e *NotAVMError) Error() string { return e.Name + " is a Mac" }

type DeleteReport = model.DeleteReport

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
	_, err := s.VM(name, "start")
	return err
}

func (s *Service) CheckStop(name string) error {
	_, err := s.VM(name, "stop")
	return err
}

func (s *Service) Start(ctx context.Context, name string) error {
	if err := s.CheckStart(name); err != nil {
		return err
	}
	if err := s.EngineUp(ctx); err != nil {
		return err
	}
	return s.daemon.Do(ctx, "start", name)
}

func (s *Service) Stop(ctx context.Context, name string) error {
	// invariant: a stop keeps the workspace, throwaway or permanent (SP-5), and
	// like every action it is refused while the engine is stopped.
	if err := s.CheckStop(name); err != nil {
		return err
	}
	if err := s.EngineUp(ctx); err != nil {
		return err
	}
	return s.daemon.Do(ctx, "stop", name)
}

func (s *Service) StopDirect(ctx context.Context, name string) error {
	// value: the stop the engine's own Stop everything takes for a run that
	// outlived the engine.
	rec, err := s.VM(name, "stop")
	if err != nil {
		return err
	}
	rec.Want = WantStopped
	if err := s.writeRecord(rec); err != nil {
		return err
	}
	if err := s.powerOff(ctx, name); err != nil {
		return err
	}
	s.retireAgent(ctx, name)
	return nil
}

func (s *Service) restartVM(ctx context.Context, name string, say func(phase, detail string)) (RestartReport, error) {
	if _, err := s.VM(name, "restart"); err != nil {
		return RestartReport{}, err
	}
	if err := s.EngineUp(ctx); err != nil {
		return RestartReport{}, err
	}
	started := s.h.Clock.Now()
	say("restarting", "poweroff, then start")
	if err := s.daemon.Do(ctx, "restart", name); err != nil {
		return RestartReport{}, err
	}
	say("back", "the engine runs it again")
	return RestartReport{Name: name, Waited: s.h.Clock.Now().Sub(started)}, nil
}

func (s *Service) Delete(ctx context.Context, name string) (DeleteReport, error) {
	if _, err := s.VM(name, "delete"); err != nil {
		return DeleteReport{}, err
	}
	if err := s.EngineUp(ctx); err != nil {
		return DeleteReport{}, err
	}
	if err := s.daemon.Do(ctx, "delete", name); err != nil {
		return DeleteReport{}, err
	}
	return DeleteReport{Name: name}, nil
}

func (s *Service) Remove(ctx context.Context, name string) (DeleteReport, error) {
	// invariant: Remove is the whole delete, run by the engine: the run stops,
	// the disk, record, files and block go, and the VM's leases and their serve
	// entries are released.
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
	if err := configFailure(s.o.SSHConfig.Remove(ctx, name), false); err != nil {
		report.Warnings = append(report.Warnings, err.Error())
	}
	return report, nil
}
