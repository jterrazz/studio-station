package machines

import (
	"errors"
	"strconv"
	"strings"

	"github.com/jterrazz/studio-station/src/catalog"
)

type RunningUnderAgentError struct{ Names []string }

func (e *RunningUnderAgentError) Error() string {
	return strings.Join(e.Names, ", ") + " still runs under its round-3 agent"
}

type DaemonDownError struct {
	Host      string
	Installed bool
}

func (e *DaemonDownError) Error() string {
	if e.Installed {
		return "the engine is stopped on " + e.Host
	}
	return "the engine is not installed on " + e.Host
}

type BadNetworkError struct {
	Value  string
	Bridge bool
}

func (e *BadNetworkError) Error() string {
	if e.Bridge {
		return "--network bridged is for macOS images: a linux workspace stays behind its Mac"
	}
	return "unknown network \"" + e.Value + "\": nat or bridged"
}

type KindRuleError struct {
	// value: a create a VM's kind refuses: Why says the rule, Fix the way out.
	Why, Fix string
}

func (e *KindRuleError) Error() string { return e.Why }

type BadCheckError struct{ Value string }

func (e *BadCheckError) Error() string {
	return "--check \"" + e.Value + "\" is neither a URL nor host:port"
}

type BadLifetimeError struct{ Value string }

func (e *BadLifetimeError) Error() string {
	return "unknown lifetime \"" + e.Value + "\": throwaway or permanent"
}

type UnknownImageError struct{ Image string }

func (e *UnknownImageError) Error() string { return "unknown image \"" + e.Image + "\"" }

type NoImageError struct{ Image string }

func (e *NoImageError) Error() string { return "image " + e.Image + " is not built on This Mac" }

type NoEngineError struct {
	Hypervisor string
	Cause      error
}

func (e *NoEngineError) Error() string { return e.Hypervisor + " is not installed or does not answer" }

func (e *NoEngineError) Unwrap() error { return e.Cause }

type TooBigError struct {
	Mac             string
	AskedGB, LeftGB int
}

func (e *TooBigError) Error() string {
	return "not enough memory on " + e.Mac + ": " + strconv.Itoa(e.AskedGB) + " GB asked, " + strconv.Itoa(e.LeftGB) + " GB left for VMs"
}

type CreateFailedError struct {
	Name, Step string
	Cause      error
}

func (e *CreateFailedError) Error() string {
	text := "creating " + e.Name + " failed at " + e.Step + ", and was rolled back"
	if e.Cause != nil {
		text += ": " + strings.TrimSpace(e.Cause.Error())
	}
	return text
}

func (e *CreateFailedError) Unwrap() error { return e.Cause }

type BadSizeError struct{ Flag, Value string }

func (e *BadSizeError) Error() string { return "--" + e.Flag + " \"" + e.Value + "\" is not a size" }

type SizeNeedsUnitError struct{ Flag, Value string }

func (e *SizeNeedsUnitError) Error() string { return "\"" + e.Value + "\" needs a unit" }

type DidNotBootError struct {
	Name  string
	Cause error
}

func (e *DidNotBootError) Error() string {
	text := e.Name + " did not boot"
	if e.Cause != nil {
		text += ": " + strings.TrimSpace(e.Cause.Error())
	}
	return text
}

type UnhealthyError struct{ Name, Step, Detail string }

func (e *UnhealthyError) Error() string {
	return e.Name + " is kept, failing " + e.Step + ": " + e.Detail
}

var ErrDaemonDown = errors.New("the engine is not running")

type NothingToChangeError struct{ Name string }

func (e *NothingToChangeError) Error() string { return "say what to change on " + e.Name }

type DiskShrinksError struct {
	Name string
	Has  int64
}

func (e *DiskShrinksError) Error() string { return "a VM's disk only grows" }

type UnknownMachineError struct{ Name string }

func (e *UnknownMachineError) Error() string { return "no machine named \"" + e.Name + "\"" }

type HereMachineError struct{ Name, Verb string }

func (e *HereMachineError) Error() string {
	if e.Verb == VerbForget {
		return "this is This Mac: station forgets only the other Macs"
	}
	return "\"" + e.Name + "\" is this machine"
}

type NotAHostError struct{ Name, Verb string }

func (e *NotAHostError) Error() string {
	if e.Verb == VerbForget {
		return e.Name + " is a VM: a VM is deleted, never forgotten"
	}
	return "\"" + e.Name + "\" is a VM, not a host"
}

type OnlyOnHostError struct{ Here catalog.MachineKind }

func (e *OnlyOnHostError) Error() string {
	return "only a host does this; here is " + e.Here.Article()
}

type NameTakenError struct {
	Name string
	VM   bool
	Host bool
	// value: the Mac a VM of that name is on, as the owner reads it.
	On string
}

func (e *NameTakenError) Error() string {
	if e.Host {
		return "\"" + e.Name + "\" already names a host"
	}
	if e.VM {
		return "\"" + e.Name + "\" is taken (on " + e.On + ")"
	}
	return "machine \"" + e.Name + "\" already exists"
}

type BadNameError struct{ Name string }

func (e *BadNameError) Error() string { return "\"" + e.Name + "\" is not a valid name" }

type UnreachableError struct{ Name string }

func (e *UnreachableError) Error() string { return e.Name + " does not answer" }

type RestartTimeoutError struct{ Name, Phase string }

func (e *RestartTimeoutError) Error() string {
	if e.Phase == PhaseStop {
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
	text := "updating the ssh config failed"
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

type NotAVMError struct{ Name, Verb string }

func (e *NotAVMError) Error() string { return e.Name + " is a Mac" }

type NotHereError struct{ Name, Host string }

func (e *NotHereError) Error() string { return e.Name + " runs on " + e.Host + ", not here" }

type StoppedError struct {
	Name     string
	Lifetime Lifetime
}

func (e *StoppedError) Error() string {
	return e.Name + " is stopped"
}

type SSHDClosedError struct {
	Name, Addr string
	Cause      error
}

func (e *SSHDClosedError) Error() string { return e.Name + "'s sshd does not answer on " + e.Addr }

func (e *SSHDClosedError) Unwrap() error { return e.Cause }

const (
	VerbForget  = "forget"
	PhaseStop   = "stop"
	PhaseReturn = "return"
)
