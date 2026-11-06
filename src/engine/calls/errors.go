package calls

import (
	"strconv"
	"strings"

	"github.com/jterrazz/studio-station/src/catalog"
)

const VMStopped = "vm_stopped"

type NotInstalledError struct{}

func (*NotInstalledError) Error() string { return "the engine is not installed" }

type StuckError struct{ VM string }

func (e *StuckError) Error() string { return e.VM + " did not stop; the engine keeps running" }

type BootError struct {
	Message string
	// value: the command line the owner types to read the engine's job.
	See   string
	Cause error
}

func (e *BootError) Error() string { return e.Message }

func (e *BootError) Unwrap() error { return e.Cause }

type DownError struct{}

func (*DownError) Error() string { return "the engine is not running" }

type RunningError struct{ PID int }

func (e *RunningError) Error() string {
	return "an engine already runs (pid " + strconv.Itoa(e.PID) + ")"
}

type HostOnlyError struct{ Here catalog.MachineKind }

func (e *HostOnlyError) Error() string {
	return "the engine runs only on a host; here is " + e.Here.Article()
}

type OpError struct {
	// value: an engine refusal worded where it is raised: its code travels
	// in the error frame, its words are the engine's.
	Code    string `json:"code"`
	Message string `json:"message"`
	Hint    string `json:"hint"`
}

func (e *OpError) Error() string { return e.Message }

func (e *OpError) Refused() bool { return strings.HasPrefix(e.Code, "unknown_") || e.Code == VMStopped }
