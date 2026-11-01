package machines

import "github.com/jterrazz/studio-station/src/catalog"

type UnknownMachineError struct{ Name string }

func (e *UnknownMachineError) Error() string { return "unknown machine \"" + e.Name + "\"" }

type HereMachineError struct{ Name string }

func (e *HereMachineError) Error() string { return "\"" + e.Name + "\" is this machine" }

type NotAHostError struct{ Name, Verb string }

func (e *NotAHostError) Error() string { return "\"" + e.Name + "\" is a VM, not a host" }

type OnlyOnHostError struct{ Here catalog.MachineKind }

func (e *OnlyOnHostError) Error() string {
	return "only a host does this; here is " + e.Here.Article()
}

type NameTakenError struct {
	Name string
	VM   bool
	Host bool
}

func (e *NameTakenError) Error() string {
	if e.Host {
		return "\"" + e.Name + "\" already names a host"
	}
	if e.VM {
		return "\"" + e.Name + "\" already names a VM"
	}
	return "machine \"" + e.Name + "\" already exists"
}

type BadNameError struct{ Name string }

func (e *BadNameError) Error() string { return "\"" + e.Name + "\" is not a valid name" }

type TemporaryRestartError struct{ Name string }

func (e *TemporaryRestartError) Error() string {
	return e.Name + " is temporary: restarting it from the host would delete it"
}

type UnreachableError struct{ Name string }

func (e *UnreachableError) Error() string { return "cannot reach \"" + e.Name + "\"" }

type RestartTimeoutError struct{ Name, Phase string }

func (e *RestartTimeoutError) Error() string {
	if e.Phase == phaseStop {
		return e.Name + " did not restart"
	}
	return e.Name + " did not come back"
}

type RestartRefusedError struct{ Name, Detail string }

func (e *RestartRefusedError) Error() string {
	return "\"" + e.Name + "\" refused to restart: " + e.Detail
}

type NeedsTerminalError struct{ Verb string }

func (e *NeedsTerminalError) Error() string { return e.Verb + " needs a terminal" }

type SSHConfigError struct {
	Path       string
	Cause      error
	RolledBack bool
}

func (e *SSHConfigError) Error() string {
	text := "ssh config update failed"
	if e.RolledBack {
		text += " (registry rolled back)"
	}
	return text + ": " + e.Cause.Error()
}

func (e *SSHConfigError) Unwrap() error { return e.Cause }

type RecordError struct {
	Path  string
	Cause error
}

func (e *RecordError) Error() string {
	return "cannot read the VM record " + e.Path + ": " + e.Cause.Error()
}
func (e *RecordError) Unwrap() error { return e.Cause }
