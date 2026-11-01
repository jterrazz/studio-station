package kit

import (
	"errors"

	"github.com/jterrazz/studio-station/src/domain/machines"
	"github.com/jterrazz/studio-station/src/fail"
)

func machinesFailure(err error) (*fail.Error, bool) {
	var unknown *machines.UnknownMachineError
	var here *machines.HereMachineError
	var notHost *machines.NotAHostError
	var onlyHost *machines.OnlyOnHostError
	var taken *machines.NameTakenError
	var bad *machines.BadNameError
	var temporary *machines.TemporaryRestartError
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
		return fail.Usage(unknown.Error(), "run station machines for the fleet"), true
	case errors.As(err, &here):
		return fail.Usage(here.Error(), "pick another machine: run station machines for the fleet"), true
	case errors.As(err, &notHost):
		return fail.Usage(notHost.Error(), notAHostHint(notHost)), true
	case errors.As(err, &onlyHost):
		return fail.Usage(onlyHost.Error(), "run it on a host"), true
	case errors.As(err, &taken):
		return fail.Usage(taken.Error(), takenHint(taken)), true
	case errors.As(err, &bad):
		return fail.Usage(bad.Error(), "use lowercase letters, digits and \"-\", 1 to 40 characters"), true
	case errors.As(err, &temporary):
		return fail.Usage(temporary.Error(), "ssh "+temporary.Name+" sudo reboot"), true
	case errors.As(err, &unreachable):
		return fail.Failed(unreachable.Error(), "station machines probe "+unreachable.Name, err), true
	case errors.As(err, &timeout):
		return fail.Failed(timeout.Error(), "station machines probe "+timeout.Name, err), true
	case errors.As(err, &refused):
		return fail.Failed(refused.Error(), "check that sudo needs no password for fdesetup on "+refused.Name, err), true
	case errors.As(err, &terminal):
		return fail.Usage(terminal.Error(), "run it on a terminal"), true
	case errors.As(err, &sshConfig):
		return fail.Failed(sshConfig.Error(), "check \""+sshConfig.Path+"\" and run it again", err), true
	case errors.As(err, &record):
		return fail.Failed(record.Error(), "repair or move away \""+record.Path+"\"", err), true
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
	var noLifetime *machines.NoLifetimeError
	var badLifetime *machines.BadLifetimeError
	var unknownImage *machines.UnknownImageError
	var noImage *machines.NoImageError
	var noEngine *machines.NoEngineError
	var tooBig *machines.TooBigError
	var badSize *machines.BadSizeError
	var stop *machines.TemporaryStopError
	var start *machines.TemporaryStartError
	var created *machines.CreateFailedError
	var down *machines.DaemonDownError
	var network *machines.BadNetworkError
	if f, ok := pipeFailure(err); ok {
		return f, true
	}
	switch {
	case errors.As(err, &down):
		return fail.Failed(down.Error(), "station setup", err), true
	case errors.As(err, &network):
		if network.Bridge {
			return fail.Usage(network.Error(), "it lets an iPhone on the Wi-Fi reach the VM; Linux VMs publish their services instead"), true
		}
		return fail.Usage(network.Error(), "station machines create --network nat"), true
	case errors.As(err, &created):
		return fail.Failed(created.Error(), "fix the cause, then run station machines create again", err), true
	case errors.As(err, &notVM):
		return fail.Usage(notVM.Error(), notAVMHint(notVM)), true
	case errors.As(err, &noLifetime):
		return fail.Usage(noLifetime.Error(), "station machines create --lifetime temporary"), true
	case errors.As(err, &badLifetime):
		return fail.Usage(badLifetime.Error(), "station machines create --lifetime temporary"), true
	case errors.As(err, &unknownImage):
		return fail.Usage(unknownImage.Error(), "station images for the images"), true
	case errors.As(err, &noImage):
		return fail.Usage(noImage.Error(), "station images build "+noImage.Image), true
	case errors.As(err, &noEngine):
		return fail.Usage(noEngine.Error(), "station tools install tart"), true
	case errors.As(err, &tooBig):
		return fail.Usage(tooBig.Error(), "ask for less: --memory 8G"), true
	case errors.As(err, &badSize):
		return fail.Usage(badSize.Error(), "write a size with its unit: 512M, 8G, 1T"), true
	case errors.As(err, &stop):
		return fail.Usage(stop.Error(), "station machines delete "+stop.Name), true
	case errors.As(err, &start):
		return fail.Usage(start.Error(), "station machines create --lifetime temporary"), true
	}
	return nil, false
}

func notAVMHint(e *machines.NotAVMError) string {
	switch e.Verb {
	case "delete":
		return "station machines forget " + e.Name
	case "stop":
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
		return fail.Usage(notHere.Error(), "ssh "+notHere.Name+" goes through "+notHere.Host+": station machines"), true
	case errors.As(err, &stopped):
		if stopped.Lifetime == machines.Temporary {
			return fail.Failed(stopped.Error(), "station machines delete "+stopped.Name, err), true
		}
		return fail.Failed(stopped.Error(), "station machines start "+stopped.Name, err), true
	case errors.As(err, &closed):
		return fail.Failed(closed.Error(), "station machines probe "+closed.Name, err), true
	case errors.As(err, &running):
		return fail.Failed(running.Error(), "stop the lasting VMs and delete the temporary ones (station machines), then station setup", err), true
	}
	return nil, false
}
