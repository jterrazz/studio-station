package machines

import (
	"context"
	"time"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/system"
)

const nodeConsole = "https://login.tailscale.com/admin/machines"

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
	target := s.agentTarget(name)
	if err := s.launchctlOK(ctx, "enable", target); err != nil {
		return err
	}
	if !s.loaded(ctx, name) {
		if err := s.launchctlOK(ctx, "bootstrap", s.guiDomain(), s.plistPath(name)); err != nil {
			return err
		}
	}
	if err := s.launchctlOK(ctx, "kickstart", target); err != nil {
		return err
	}
	_, err := s.awaitOnline(ctx, name)
	return err
}

func (s *Service) Stop(ctx context.Context, name string) error {
	// invariant: a stopped lasting VM stays stopped across logins, because its
	// agent is disabled; the node stays, a lasting VM keeps its identity.
	if err := s.CheckStop(name); err != nil {
		return err
	}
	cfg, err := s.store.Load()
	if err != nil {
		return err
	}
	if err := s.powerOff(ctx, name, s.vmUser(cfg)); err != nil {
		return err
	}
	return s.launchctlOK(ctx, "disable", s.agentTarget(name))
}

func (s *Service) restartVM(ctx context.Context, name string, say func(phase, detail string)) (RestartReport, error) {
	rec, err := s.VM(name, "restart")
	if err != nil {
		return RestartReport{}, err
	}
	if rec.Lifetime == Temporary {
		return RestartReport{}, &TemporaryRestartError{Name: name}
	}
	cfg, err := s.store.Load()
	if err != nil {
		return RestartReport{}, err
	}
	started := s.h.Clock.Now()
	say("restarting", "poweroff, then start")
	if err := s.powerOff(ctx, name, s.vmUser(cfg)); err != nil {
		return RestartReport{}, err
	}
	say("down", "the VM is off")
	if err := s.launchctlOK(ctx, "kickstart", s.agentTarget(name)); err != nil {
		return RestartReport{}, err
	}
	say("back", "its agent runs it again")
	return RestartReport{Name: name, Waited: s.h.Clock.Now().Sub(started)}, nil
}

func (s *Service) Delete(ctx context.Context, name string) (DeleteReport, error) {
	// invariant: run by the VM's own agent (its run ended), a delete skips the
	// stop and boots the agent out last, since that ends the agent itself.
	rec, err := s.VM(name, "delete")
	if err != nil {
		return DeleteReport{}, err
	}
	cfg, err := s.store.Load()
	if err != nil {
		return DeleteReport{}, err
	}
	self := s.o.Agent == name
	report := DeleteReport{Name: name}
	if rec.NodeID != "" {
		if err := s.tn.Remove(ctx, rec.NodeID); err != nil && !s.natLogout(ctx, name, s.vmUser(cfg)) {
			report.Warnings = append(report.Warnings, "the node of "+name+" is still on the tailnet: remove it at "+nodeConsole)
		}
	}
	if !self {
		_, _ = s.launchctl(ctx, "bootout", s.agentTarget(name))
		if s.engineState(ctx, name) == system.VMRunning {
			if err := s.engine.Stop(ctx, name, hardStopWait); err != nil {
				return report, err
			}
		}
	}
	if s.engineState(ctx, name) != system.VMMissing {
		if err := s.engine.Delete(ctx, name); err != nil {
			return report, err
		}
	}
	s.removeFiles(name, true)
	if err := s.editConfig(nil, false, []string{name}); err != nil {
		report.Warnings = append(report.Warnings, err.Error())
	}
	if self {
		_, _ = s.launchctl(ctx, "bootout", s.agentTarget(name))
	}
	return report, nil
}

// hazard: the logout runs over the NAT with the image key, never over the
// tailnet: logging out cuts the very session that would carry it.
func (s *Service) natLogout(ctx context.Context, name, user string) bool {
	if s.engineState(ctx, name) != system.VMRunning {
		return false
	}
	ip, err := s.engine.IP(ctx, name, natIPWait)
	if err != nil {
		return false
	}
	out, err := s.nat(ctx, ip, user, "", 30*time.Second, "sudo", "tailscale", "logout")
	return err == nil && out.ExitCode == 0
}
