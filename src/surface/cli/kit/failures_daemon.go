package kit

import (
	"errors"

	"github.com/jterrazz/studio-station/src/core/fail"
	"github.com/jterrazz/studio-station/src/domain/guest"
	"github.com/jterrazz/studio-station/src/domain/services"
	"github.com/jterrazz/studio-station/src/engine"
)

func daemonFailure(err error) (*fail.Error, bool) {
	var running *engine.RunningError
	var hostOnly *engine.HostOnlyError
	var down *engine.DownError
	var guestHost *guest.HostOnlyRefusedError
	if f, ok := controlFailure(err); ok {
		return f, true
	}
	switch {
	case errors.As(err, &running):
		return usage(running.Error(), "station overview shows it"), true
	case errors.As(err, &hostOnly):
		return usage(hostOnly.Error(), "run it on a host: station setup installs it"), true
	case errors.As(err, &down):
		return failed(down.Error(), "station setup", err), true
	case errors.As(err, &guestHost):
		return usage(guestHost.Error(), "the host's daemon runs it inside each VM"), true
	}
	return servicesFailure(err)
}

func servicesFailure(err error) (*fail.Error, bool) {
	var full *services.RangeFullError
	var perVM *services.MaxPerVMError
	var noLease *services.NoLeaseError
	var badPort *services.BadPortError
	var badTarget *services.BadTargetError
	var ignored *services.IgnoredPortError
	var away *services.HostAwayError
	var pinned *services.PinnedError
	var vmOnly *services.VMOnlyError
	var unknown *services.UnknownVMError
	var hereInVM *services.HereInVMError
	switch {
	case errors.As(err, &hereInVM):
		return usage(hereInVM.Error(), "station services"), true
	case errors.As(err, &full):
		return failed(full.Error(), "station services release <port>", err), true
	case errors.As(err, &perVM):
		return failed(perVM.Error(), "station services release <port>", err), true
	case errors.As(err, &noLease):
		return usage(noLease.Error(), "station services"), true
	case errors.As(err, &badPort):
		return usage(badPort.Error(), "a port is a number from 1 to 65535"), true
	case errors.As(err, &badTarget):
		if badTarget.InVM {
			return usage(badTarget.Error(), "station services release <port>"), true
		}
		return usage(badTarget.Error(), "station services release <vm>:<port>"), true
	case errors.As(err, &ignored):
		return usage(ignored.Error(), "the host ignores ports below 1024 and 22"), true
	case errors.As(err, &away):
		return failed(away.Error(), away.Hint(), err), true
	case errors.As(err, &pinned):
		return usage(pinned.Error(), "re-run with --yes"), true
	case errors.As(err, &vmOnly):
		return usage(vmOnly.Error(), "station services url runs inside a VM; here: station services"), true
	case errors.As(err, &unknown):
		return usage(unknown.Error(), "station services"), true
	}
	return nil, false
}

func controlFailure(err error) (*fail.Error, bool) {
	var notInstalled *engine.NotInstalledError
	var offHost *engine.OffHostError
	var stuck *engine.StuckError
	var noAnswer *engine.NoAnswerError
	var boot *engine.BootError
	switch {
	case errors.As(err, &notInstalled):
		return failed(notInstalled.Error(), "station setup", err), true
	case errors.As(err, &offHost):
		return usage(offHost.Error(), "ssh <host> station "+offHost.Verb), true
	case errors.As(err, &stuck):
		return failed(stuck.Error(), "station machines delete "+stuck.VM, err), true
	case errors.As(err, &noAnswer):
		return failed(noAnswer.Error(), "tail ~/.local/state/station/logs/daemon.log", err), true
	case errors.As(err, &boot):
		return failed(boot.Error(), boot.See, err), true
	}
	return nil, false
}

// invariant: a read of a VM that is not running is a finding, the VM named
// with the verb that starts it; an action there is refused.
func vmStopped(err error) bool {
	var e *fail.Error
	var op *engine.OpError
	return errors.As(err, &e) && e.Code == engine.VMStopped || errors.As(err, &op) && op.Code == engine.VMStopped
}
