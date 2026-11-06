package kit

import (
	"errors"
	"github.com/jterrazz/studio-station/src/ports"

	"github.com/jterrazz/studio-station/src/core/fail"
	"github.com/jterrazz/studio-station/src/domain/guest"
	"github.com/jterrazz/studio-station/src/domain/services"
	"github.com/jterrazz/studio-station/src/engine"
)

func engineFailure(err error) (*fail.Error, bool) {
	var running *engine.RunningError
	var macOnly *engine.MacOnlyError
	var down *engine.DownError
	var guestMacOnly *guest.HostOnlyRefusedError
	if f, ok := controlFailure(err); ok {
		return f, true
	}
	switch {
	case errors.As(err, &running):
		return usage(running.Error(), "station overview shows it"), true
	case errors.As(err, &macOnly):
		return usage(macOnly.Error(), "run it on a Mac: station setup installs it"), true
	case errors.As(err, &down):
		return failed(down.Error(), "station setup", err), true
	case errors.As(err, &guestMacOnly):
		return usage(guestMacOnly.Error(), "its Mac's engine runs it inside each VM"), true
	}
	return portsFailure(err)
}

func portsFailure(err error) (*fail.Error, bool) {
	var full *ports.RangeFull
	var perVM *ports.VMFull
	var badPort *services.BadPortError
	var ignored *services.IgnoredPortError
	var away *services.HostAwayError
	var unknown *services.UnknownVMError
	switch {
	case errors.As(err, &full):
		return failed(full.Error(), "station ports stop-sharing <vm> <port>", err), true
	case errors.As(err, &perVM):
		return failed(perVM.Error(), "station ports stop-sharing "+perVM.VM+" <port>", err), true
	case errors.As(err, &badPort):
		return usage(badPort.Error(), "a port is a number from 1 to 65535"), true
	case errors.As(err, &ignored):
		return usage(ignored.Error(), "station never shares port 22 nor a port below 1024"), true
	case errors.As(err, &away):
		return failed(away.Error(), away.Hint(), err), true
	case errors.As(err, &unknown):
		return usage(unknown.Error(), "station machines"), true
	}
	return nil, false
}

func controlFailure(err error) (*fail.Error, bool) {
	var notInstalled *engine.NotInstalledError
	var stuck *engine.StuckError
	var boot *engine.BootError
	switch {
	case errors.As(err, &notInstalled):
		return failed(notInstalled.Error(), "station setup", err), true
	case errors.As(err, &stuck):
		return failed(stuck.Error(), "station machines delete "+stuck.VM, err), true
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
