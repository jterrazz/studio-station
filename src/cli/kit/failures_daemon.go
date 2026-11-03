package kit

import (
	"errors"

	"github.com/jterrazz/studio-station/src/core/fail"
	"github.com/jterrazz/studio-station/src/domain/daemon"
	"github.com/jterrazz/studio-station/src/domain/guest"
	"github.com/jterrazz/studio-station/src/domain/services"
)

func daemonFailure(err error) (*fail.Error, bool) {
	var running *daemon.RunningError
	var hostOnly *daemon.HostOnlyError
	var down *daemon.DownError
	var op *daemon.OpError
	var guestHost *guest.HostOnlyRefusedError
	if f, ok := controlFailure(err); ok {
		return f, true
	}
	switch {
	case errors.As(err, &running):
		return fail.Usage(running.Error(), "station status shows it"), true
	case errors.As(err, &hostOnly):
		return fail.Usage(hostOnly.Error(), "run it on a host: station setup installs it"), true
	case errors.As(err, &down):
		return fail.Failure(down.Error(), "station setup", err), true
	case errors.As(err, &guestHost):
		return fail.Usage(guestHost.Error(), "the host's daemon runs it inside each VM"), true
	case errors.As(err, &op):
		switch op.Code {
		case "unknown_vm", "unknown_op":
			return fail.Usage(op.Message, op.Hint), true
		}
		return fail.Failure(op.Message, op.Hint, err), true
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
	var remote *services.RemoteError
	var hereInVM *services.HereInVMError
	switch {
	case errors.As(err, &hereInVM):
		return fail.Usage(hereInVM.Error(), "station services"), true
	case errors.As(err, &full):
		return fail.Failure(full.Error(), "station services release <port>", err), true
	case errors.As(err, &perVM):
		return fail.Failure(perVM.Error(), "station services release <port>", err), true
	case errors.As(err, &noLease):
		return fail.Usage(noLease.Error(), "station services"), true
	case errors.As(err, &badPort):
		return fail.Usage(badPort.Error(), "a port is a number from 1 to 65535"), true
	case errors.As(err, &badTarget):
		if badTarget.InVM {
			return fail.Usage(badTarget.Error(), "station services release <port>"), true
		}
		return fail.Usage(badTarget.Error(), "station services release <vm>:<port>"), true
	case errors.As(err, &ignored):
		return fail.Usage(ignored.Error(), "the host ignores ports below 1024 and 22"), true
	case errors.As(err, &away):
		return fail.Failure(away.Error(), away.Hint(), err), true
	case errors.As(err, &pinned):
		return fail.Usage(pinned.Error(), "re-run with --yes"), true
	case errors.As(err, &vmOnly):
		return fail.Usage(vmOnly.Error(), "station services url runs inside a VM; here: station services"), true
	case errors.As(err, &unknown):
		return fail.Usage(unknown.Error(), "station services"), true
	case errors.As(err, &remote):
		return fail.Silent(fail.Exit(remote.Code)), true
	}
	return nil, false
}

func controlFailure(err error) (*fail.Error, bool) {
	var notInstalled *daemon.NotInstalledError
	var offHost *daemon.OffHostError
	var unconfirmed *daemon.UnconfirmedError
	var stuck *daemon.StuckError
	var noAnswer *daemon.NoAnswerError
	var boot *daemon.BootError
	switch {
	case errors.As(err, &notInstalled):
		return fail.Failure(notInstalled.Error(), "station setup", err), true
	case errors.As(err, &offHost):
		return fail.Usage(offHost.Error(), "ssh <host> station "+offHost.Verb), true
	case errors.As(err, &unconfirmed):
		return fail.Usage(unconfirmed.Error(), "station stop --yes"), true
	case errors.As(err, &stuck):
		return fail.Failure(stuck.Error(), "station machines delete "+stuck.VM, err), true
	case errors.As(err, &noAnswer):
		return fail.Failure(noAnswer.Error(), "tail ~/.local/state/station/logs/daemon.log", err), true
	case errors.As(err, &boot):
		return fail.Failure(boot.Error(), "launchctl print gui/$(id -u)/"+daemon.Label, err), true
	}
	return nil, false
}
