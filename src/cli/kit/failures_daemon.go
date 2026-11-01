package kit

import (
	"errors"

	"github.com/jterrazz/studio-station/src/domain/daemon"
	"github.com/jterrazz/studio-station/src/domain/guest"
	"github.com/jterrazz/studio-station/src/domain/services"
	"github.com/jterrazz/studio-station/src/fail"
)

func daemonFailure(err error) (*fail.Error, bool) {
	var running *daemon.RunningError
	var hostOnly *daemon.HostOnlyError
	var down *daemon.DownError
	var op *daemon.OpError
	var guestHost *guest.HostOnlyRefusedError
	switch {
	case errors.As(err, &running):
		return fail.Usage(running.Error(), "station status shows it"), true
	case errors.As(err, &hostOnly):
		return fail.Usage(hostOnly.Error(), "run it on a host: station setup installs it"), true
	case errors.As(err, &down):
		return fail.Failed(down.Error(), "station setup", err), true
	case errors.As(err, &guestHost):
		return fail.Usage(guestHost.Error(), "the host's daemon runs it inside each VM"), true
	case errors.As(err, &op):
		switch op.Code {
		case "unknown_vm", "unknown_op":
			return fail.Usage(op.Message, op.Hint), true
		}
		return fail.Failed(op.Message, op.Hint, err), true
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
	switch {
	case errors.As(err, &full):
		return fail.Failed(full.Error(), "station services release <port>", err), true
	case errors.As(err, &perVM):
		return fail.Failed(perVM.Error(), "station services release <port>", err), true
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
		return fail.Failed(away.Error(), "on "+away.Host+": station setup", err), true
	case errors.As(err, &pinned):
		return fail.Usage(pinned.Error(), "re-run with --yes"), true
	case errors.As(err, &vmOnly):
		return fail.Usage(vmOnly.Error(), "station services url runs inside a VM; here: station services"), true
	case errors.As(err, &unknown):
		return fail.Usage(unknown.Error(), "station services"), true
	case errors.As(err, &remote):
		return fail.Silent(fail.Code(remote.Code)), true
	}
	return nil, false
}
