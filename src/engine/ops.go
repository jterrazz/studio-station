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
	model "github.com/jterrazz/studio-station/src/machines"
)

const Label = catalog.EngineLabel

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
	// value: the process that holds its guest on this Mac, 0 while none does.
	Holder int    `json:"holder,omitempty"`
	Guest  string `json:"guest"`
	IP     string `json:"ip"`
	Detail string `json:"detail"`
	// value: its ssh connection opened, whether station's guest answers on
	// it or not; a server's restarts and health check.
	SSH      bool              `json:"ssh,omitempty"`
	Restarts model.Restarts    `json:"restarts,omitzero"`
	Check    model.CheckStatus `json:"check,omitzero"`
}

type Peer interface {
	// value: what a process asks of this Mac's engine over its socket.
	Hello(ctx context.Context) (Hello, error)
	VMs(ctx context.Context) ([]VMView, error)
	StopAll(ctx context.Context) (StopAll, error)
	Power(ctx context.Context) (power.View, error)
}

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

func SocketPath(state string) string { return path.Join(state, "engine.sock") }

func LogPath(state string) string { return path.Join(state, "logs", "engine.log") }

func ChildrenDir(state string) string { return path.Join(state, "children") }

type OpError struct {
	// value: an engine refusal worded where it is raised: its code travels
	// in the error frame, its words are the engine's.
	Code    string `json:"code"`
	Message string `json:"message"`
	Hint    string `json:"hint"`
}

func (e *OpError) Error() string { return e.Message }

func (e *OpError) Refused() bool { return strings.HasPrefix(e.Code, "unknown_") || e.Code == VMStopped }

func fault(code, message, hint string) *OpError {
	// value: a code the caller got wrong (a VM, a verb, a page it named), not
	// one the engine failed at.
	return &OpError{Code: code, Message: message, Hint: hint}
}

func (d *Engine) Hello() Hello {
	return Hello{Version: d.o.Version, PID: d.o.PID, StartedAt: d.started, Executable: d.o.Executable}
}

func (d *Engine) VMs() []VMView { return d.vmViews() }

func (d *Engine) Services(vm string) services.HostView { return d.hostView(vm) }

func (d *Engine) Reserve(ctx context.Context, vm string, port int, pinned bool, profile string) (services.View, error) {
	return d.shareOf(ctx, vm, port, pinned, profile)
}

func (d *Engine) Release(ctx context.Context, vm string, port int) (services.Lease, error) {
	return d.stopSharing(ctx, vm, port)
}

// invariant: a share takes back a stop of sharing, and a stop of sharing is
// remembered, on this Mac whoever asked: a command here or a VM's guest.
func (d *Engine) shareOf(ctx context.Context, vm string, port int, pinned bool, profile string) (services.View, error) {
	if d.o.Remember != nil {
		if err := d.o.Remember(vm, port, false); err != nil {
			return services.View{}, err
		}
	}
	return d.reserve(ctx, vm, port, pinned, profile)
}

func (d *Engine) stopSharing(ctx context.Context, vm string, port int) (services.Lease, error) {
	l, err := d.release(ctx, vm, port)
	if err != nil || d.o.Remember == nil {
		return l, err
	}
	return l, d.o.Remember(vm, port, true)
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

func (d *Engine) VM(ctx context.Context, verb, vm string) error {
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

func (d *Engine) Accounts(ctx context.Context) AccountsView { return d.accounts(ctx) }

func (d *Engine) StopAll(ctx context.Context) StopAll {
	shared := d.sharedVMs()
	vms := d.stopAll(ctx)
	all := StopAll{VMs: vms, PowerEnded: d.power.Stop()}
	for _, v := range vms {
		all.SharingEnded = all.SharingEnded || v.Result == ResultStopped && shared[v.Name]
	}
	return all
}

func (d *Engine) Power() power.View { return d.power.View() }

type AwakeIn struct {
	Lid     bool `json:"lid,omitempty"`
	Battery bool `json:"battery,omitempty"`
	// value: seconds.
	For int    `json:"for,omitempty"`
	By  string `json:"by,omitempty"`
}

func (d *Engine) Awake(r AwakeIn) (power.Awoken, error) { return d.powerAwake(r) }

func (d *Engine) Sleep() PowerSlept { return d.powerSleep() }

func (d *Engine) Refresh(pages []string) error {
	if err := d.CheckPages(pages); err != nil {
		return err
	}
	d.hub.refreshed(pages)
	return nil
}
