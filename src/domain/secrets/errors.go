package secrets

type NotSessionError struct{ Name string }

func (e *NotSessionError) Error() string {
	return "\"" + e.Name + "\" cannot be lent: only session:<id> names and " + ClaudeToken + " can"
}

type NotOnLendListError struct{ Name string }

func (e *NotOnLendListError) Error() string { return "\"" + e.Name + "\" is not on the lend list" }

type NotLentError struct{ Name, VM string }

func (e *NotLentError) Error() string {
	return "\"" + e.Name + "\" is not lent to \"" + e.VM + "\""
}

type UnknownVMError struct{ Name string }

func (e *UnknownVMError) Error() string { return "unknown VM \"" + e.Name + "\"" }

type NotAVMError struct{ Name string }

func (e *NotAVMError) Error() string {
	return "\"" + e.Name + "\" is a host: secrets are lent to VMs only"
}

type OffHostError struct{}

func (e *OffHostError) Error() string { return "secrets are lent from a host, not from a VM" }

type EmptyLendListError struct{}

func (e *EmptyLendListError) Error() string { return "the lend list is empty: nothing to lend" }

type NoStudioError struct{ VM string }

func (e *NoStudioError) Error() string { return "no studio command in \"" + e.VM + "\"" }

type StepError struct {
	VM, Step string
	Cause    error
}

func (e *StepError) Error() string {
	msg := e.Step + " failed for \"" + e.VM + "\""
	if e.Cause != nil {
		msg += ": " + e.Cause.Error()
	}
	return msg
}

func (e *StepError) Unwrap() error { return e.Cause }
