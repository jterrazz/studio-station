package engine

import (
	"context"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/domain/power"
	"github.com/jterrazz/studio-station/src/domain/services"
)

const Label = catalog.DaemonLabel

type Hello struct {
	Version    string    `json:"version"`
	PID        int       `json:"pid"`
	StartedAt  time.Time `json:"started_at"`
	Executable string    `json:"executable,omitempty"`
	// value: the names of the calls this engine serves, as calls.json lists them.
	Calls []string `json:"calls,omitempty"`
}

type VMView struct {
	Name   string `json:"name"`
	State  string `json:"state"`
	RunPID int    `json:"run_pid"`
	Guest  string `json:"guest"`
	IP     string `json:"ip"`
	Detail string `json:"detail"`
}

type Peer interface {
	// value: what a process asks of this Mac's engine over its socket.
	Hello(ctx context.Context) (Hello, error)
	VMs(ctx context.Context) ([]VMView, error)
	StopAll(ctx context.Context) (StopAll, error)
}

type DownError struct{}

func (*DownError) Error() string { return "the station daemon is not running" }

type RunningError struct{ PID int }

func (e *RunningError) Error() string {
	return "a station daemon already runs (pid " + strconv.Itoa(e.PID) + ")"
}

type HostOnlyError struct{ Here catalog.MachineKind }

func (e *HostOnlyError) Error() string {
	return "the daemon runs only on a host; here is " + e.Here.Article()
}

func SocketPath(state string) string { return path.Join(state, "daemon.sock") }

func ChildrenDir(state string) string { return path.Join(state, "children") }

type OpError struct {
	// value: an engine refusal worded where it is raised: its code travels
	// in the error frame, its words are the engine's.
	Code    string `json:"code"`
	Message string `json:"message"`
	Hint    string `json:"hint"`
}

func (e *OpError) Error() string { return e.Message }

func (e *OpError) Refused() bool { return strings.HasPrefix(e.Code, "unknown_") }

func fault(code, message, hint string) *OpError {
	// value: a code the caller got wrong (a VM, a verb, a page it named), not
	// one the engine failed at.
	return &OpError{Code: code, Message: message, Hint: hint}
}

func (d *Daemon) Hello() Hello {
	return Hello{Version: d.o.Version, PID: d.o.PID, StartedAt: d.started, Executable: d.o.Executable}
}

func (d *Daemon) VMs() []VMView { return d.vmViews() }

func (d *Daemon) Services(vm string) services.HostView { return d.hostView(vm) }

func (d *Daemon) Reserve(ctx context.Context, vm string, port int, pinned bool, profile string) (services.View, error) {
	return d.reserve(ctx, vm, port, pinned, profile)
}

func (d *Daemon) Release(ctx context.Context, vm string, port int) (services.Lease, error) {
	return d.release(ctx, vm, port)
}

// value: what a VM's verb asks of the engine, by the verb's name.
const (
	VMRun     = "run"
	VMStart   = "start"
	VMStop    = "stop"
	VMRestart = "restart"
	VMDelete  = "delete"
	VMForget  = "forget"
)

func VMVerbs() []string { return []string{VMRun, VMStart, VMStop, VMRestart, VMDelete, VMForget} }

func (d *Daemon) VM(ctx context.Context, verb, vm string) error {
	switch verb {
	case VMRun:
		if err := d.opRun(ctx, vm); err != nil {
			return err
		}
		return d.awaitGuest(ctx, vm)
	case VMStart:
		if err := d.opStart(ctx, vm); err != nil {
			return err
		}
		return d.awaitGuest(ctx, vm)
	case VMStop:
		return d.opStop(ctx, vm, true)
	case VMRestart:
		if err := d.opStop(ctx, vm, false); err != nil {
			return err
		}
		if err := d.opRun(ctx, vm); err != nil {
			return err
		}
		return d.awaitGuest(ctx, vm)
	case VMDelete:
		return d.opDelete(ctx, vm)
	case VMForget:
		d.forget(vm)
		return nil
	}
	return fault("unknown_op", "unknown verb \""+verb+"\" for a VM", "station machines")
}

func (d *Daemon) Accounts(ctx context.Context) AccountsView { return d.accounts(ctx) }

func (d *Daemon) StopAll(ctx context.Context) StopAll {
	vms := d.stopAll(ctx)
	return StopAll{VMs: vms, PowerEnded: d.power.Stop()}
}

func (d *Daemon) Power() power.View { return d.power.View() }

type AwakeIn struct {
	Lid     bool `json:"lid,omitempty"`
	Battery bool `json:"battery,omitempty"`
	// value: seconds.
	For int    `json:"for,omitempty"`
	By  string `json:"by,omitempty"`
}

func (d *Daemon) Awake(r AwakeIn) (power.Awoken, error) { return d.powerAwake(r) }

func (d *Daemon) Sleep() PowerSlept { return d.powerSleep() }

func (d *Daemon) Refresh(pages []string) error {
	if err := d.CheckPages(pages); err != nil {
		return err
	}
	d.hub.refreshed()
	return nil
}
