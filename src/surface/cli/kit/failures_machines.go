package kit

import (
	"errors"
	"strconv"

	"github.com/jterrazz/studio-station/src/core/fail"
	"github.com/jterrazz/studio-station/src/domain/machines"
	"github.com/jterrazz/studio-station/src/surface/cli/words"
)

func machinesFailure(err error) (*fail.Error, bool) {
	var unknown *machines.UnknownMachineError
	var here *machines.HereMachineError
	var notHost *machines.NotAHostError
	var onlyHost *machines.OnlyOnHostError
	var taken *machines.NameTakenError
	var bad *machines.BadNameError
	var unreachable *machines.UnreachableError
	var timeout *machines.RestartTimeoutError
	var refused *machines.RestartRefusedError
	var terminal *machines.NeedsTerminalError
	var sshConfig *machines.SSHConfigError
	var record *machines.RecordError
	if f, ok := vmFailure(err); ok {
		return f, true
	}
	switch {
	case errors.As(err, &unknown):
		return usage(unknown.Error(), "station machines"), true
	case errors.As(err, &here):
		return usage(here.Error(), "pick another machine: run station machines for the fleet"), true
	case errors.As(err, &notHost):
		return usage(notHost.Error(), notAHostHint(notHost)), true
	case errors.As(err, &onlyHost):
		return usage(onlyHost.Error(), "run it on a host"), true
	case errors.As(err, &taken):
		return usage(taken.Error(), takenHint(taken)), true
	case errors.As(err, &bad):
		return usage(bad.Error(), "use lowercase letters, digits and \"-\", 1 to 40 characters"), true
	case errors.As(err, &unreachable):
		return failed(unreachable.Error(), "station machines", err), true
	case errors.As(err, &timeout):
		return failed(timeout.Error(), "station machines "+timeout.Name, err), true
	case errors.As(err, &refused):
		return failed(refused.Error(), "check that sudo needs no password for fdesetup on "+refused.Name, err), true
	case errors.As(err, &terminal):
		return Translate(fail.New(fail.NeedsTerminal, words.NeedsTerminal, terminal.Verb, nil), ""), true
	case errors.As(err, &sshConfig):
		return failed(sshConfig.Error(), "check \""+sshConfig.Path+"\" and run it again", err), true
	case errors.As(err, &record):
		return failed(record.Error(), "repair or move away \""+record.Path+"\"", err), true
	}
	return nil, false
}

func notAHostHint(e *machines.NotAHostError) string {
	if e.Verb == "forget" {
		return "station machines delete " + e.Name
	}
	return "run station machines for the hosts"
}

func takenHint(e *machines.NameTakenError) string {
	if e.VM || e.Host {
		return "pick another name"
	}
	return "station machines forget " + e.Name + " first, to change it"
}

func vmFailure(err error) (*fail.Error, bool) {
	var notVM *machines.NotAVMError
	var badLifetime *machines.BadLifetimeError
	var unknownImage *machines.UnknownImageError
	var noImage *machines.NoImageError
	var noEngine *machines.NoEngineError
	var tooBig *machines.TooBigError
	var badSize *machines.BadSizeError
	var noUnit *machines.SizeNeedsUnitError
	var created *machines.CreateFailedError
	var down *machines.DaemonDownError
	var network *machines.BadNetworkError
	var nothing *machines.NothingToChangeError
	var shrinks *machines.DiskShrinksError
	if f, ok := pipeFailure(err); ok {
		return f, true
	}
	switch {
	case errors.As(err, &down):
		if down.Installed {
			return failed(down.Error(), "station engine start", err), true
		}
		return failed(down.Error(), "station setup", err), true
	case errors.As(err, &nothing):
		return usage(nothing.Error(), "station machines edit "+nothing.Name+" --memory 8G"), true
	case errors.As(err, &shrinks):
		return usage(shrinks.Error(), shrinks.Name+" has "+strconv.FormatFloat(float64(shrinks.Has)/1e9, 'f', 1, 64)+" GB: ask for more"), true
	case errors.As(err, &network):
		if network.Bridge {
			return usage(network.Error(), "share its ports instead: station ports share"), true
		}
		return usage(network.Error(), "station machines create, behind its Mac"), true
	case errors.As(err, &created):
		return failed(created.Error(), "fix the cause, then run station machines create again", err), true
	case errors.As(err, &notVM):
		return usage(notVM.Error(), notAVMHint(notVM)), true
	case errors.As(err, &badLifetime):
		return usage(badLifetime.Error(), "station machines create --permanent"), true
	case errors.As(err, &unknownImage):
		return usage(unknownImage.Error(), "station images for the images"), true
	case errors.As(err, &noImage):
		return usage(noImage.Error(), "station images build "+noImage.Image), true
	case errors.As(err, &noEngine):
		return usage(noEngine.Error(), "station tools install "+noEngine.Hypervisor), true
	case errors.As(err, &tooBig):
		if tooBig.LeftGB < 1 {
			return usage(tooBig.Error(), "stop a VM first: station machines"), true
		}
		return usage(tooBig.Error(), "ask for less: --memory "+strconv.Itoa(tooBig.LeftGB)+"G"), true
	case errors.As(err, &noUnit):
		return usage(noUnit.Error(), "--"+noUnit.Flag+" "+noUnit.Value+"G"), true
	case errors.As(err, &badSize):
		return usage(badSize.Error(), "write a size with its unit: 512M, 8G, 1T"), true
	}
	return nil, false
}

func notAVMHint(e *machines.NotAVMError) string {
	switch e.Verb {
	case "delete", "stop":
		return "station machines restart " + e.Name
	}
	return "run station machines for the VMs"
}

func pipeFailure(err error) (*fail.Error, bool) {
	var notHere *machines.NotHereError
	var stopped *machines.StoppedError
	var closed *machines.SSHDClosedError
	var running *machines.RunningUnderAgentError
	switch {
	case errors.As(err, &notHere):
		return usage(notHere.Error(), "ssh "+notHere.Name+" goes through "+notHere.Host+": station machines"), true
	case errors.As(err, &stopped):
		return failed(stopped.Error(), "station machines start "+stopped.Name, err), true
	case errors.As(err, &closed):
		return failed(closed.Error(), "station machines "+closed.Name, err), true
	case errors.As(err, &running):
		return failed(running.Error(), "stop the running VMs (station machines), then station setup", err), true
	}
	return nil, false
}
