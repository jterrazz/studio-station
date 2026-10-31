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
	switch {
	case errors.As(err, &unknown):
		return fail.Usage(unknown.Error(), "station machines lists the fleet"), true
	case errors.As(err, &here):
		return fail.Usage(here.Error(), "add --on to act on another machine, or run it there"), true
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
	return "station machines lists the hosts"
}

func takenHint(e *machines.NameTakenError) string {
	if e.VM {
		return "pick another name"
	}
	return "station machines forget " + e.Name + " first, to change it"
}
