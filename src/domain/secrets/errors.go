package secrets

import "github.com/jterrazz/studio-station/src/catalog"

type NotAVMError struct{ Name string }

func (e *NotAVMError) Error() string {
	return e.Name + " is a Mac: accounts are shared with VMs only"
}

type OffHostError struct{ Here catalog.MachineKind }

func (e *OffHostError) Error() string {
	return "accounts are shared from a Mac; here is " + e.Here.Article()
}

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
