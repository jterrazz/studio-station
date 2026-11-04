package process

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jterrazz/studio-station/src/core/call"
	"github.com/jterrazz/studio-station/src/core/fail"
	"github.com/jterrazz/studio-station/src/core/machine"
	"github.com/jterrazz/studio-station/src/engine"
	"github.com/jterrazz/studio-station/src/remote/serve"
)

// value: the names of the engine's own calls: what a process of this Mac
// asks of the engine's live state (its runs, its leases, its guest
// connections, its Power), served beside the registry and never in calls.json.
const (
	Hello     = "engine.hello"
	VMs       = "engine.vms"
	Services  = "engine.services"
	Reserve   = "engine.reserve"
	Release   = "engine.release"
	GuestExec = "engine.guest-exec"
	Accounts  = "engine.accounts"
	StopAll   = "engine.stop-all"
	Power     = "engine.power"
	Awake     = "engine.power-awake"
	Sleep     = "engine.power-sleep"
	Refresh   = "engine.refresh"
	// value: the watch of the engine's pages.
	Page = "page"
)

func VM(verb string) string { return "engine.vm-" + verb }

type VMIn struct {
	VM string `json:"vm"`
}

type ReserveIn struct {
	VM        string `json:"vm"`
	GuestPort int    `json:"guest_port"`
	Pinned    bool   `json:"pinned,omitempty"`
	Profile   string `json:"profile,omitempty"`
}

type ExecIn struct {
	VM   string `json:"vm"`
	Line string `json:"line"`
	// invariant: what a command inside a VM reads, a secret included: it travels
	// in the input alone, never in Line.
	Stdin string `json:"stdin,omitempty"`
}

type ExecOut struct {
	Code   int    `json:"code"`
	Stdout string `json:"stdout"`
	Stderr string `json:"stderr"`
}

type PagesIn struct {
	Pages []string `json:"pages"`
}

func own[In any](d *engine.Daemon, name string, run func(ctx context.Context, d *engine.Daemon, in In) (any, error)) call.Handler {
	return call.Bind(call.Call[*engine.Daemon, In, any]{
		Name: name, Where: call.InEngine, Applies: machine.Anywhere,
		Run: func(ctx context.Context, d *engine.Daemon, in In, _ call.Progress) (any, error) {
			out, err := run(ctx, d, in)
			return out, worded(err)
		},
	}, d)
}

// invariant: an engine refusal leaves as the error it is, typed by whether
// the caller named something unknown, its words the engine's.
func worded(err error) error {
	var op *engine.OpError
	if !errors.As(err, &op) {
		return err
	}
	t := fail.Failed
	if op.Refused() {
		t = fail.Refused
	}
	return &fail.Error{Type: t, Code: fail.Code(op.Code), Message: op.Message, Hint: op.Hint}
}

func calls(d *engine.Daemon, served []string) []call.Handler {
	out := make([]call.Handler, 0, 12+len(engine.VMVerbs()))
	out = append(out,
		own(d, Hello, func(_ context.Context, d *engine.Daemon, _ struct{}) (any, error) {
			h := d.Hello()
			h.Calls = served
			return h, nil
		}),
		own(d, VMs, func(_ context.Context, d *engine.Daemon, _ struct{}) (any, error) { return d.VMs(), nil }),
		own(d, Services, func(_ context.Context, d *engine.Daemon, in VMIn) (any, error) { return d.Services(in.VM), nil }),
		own(d, Reserve, func(ctx context.Context, d *engine.Daemon, in ReserveIn) (any, error) {
			return d.Reserve(ctx, in.VM, in.GuestPort, in.Pinned, in.Profile)
		}),
		own(d, Release, func(ctx context.Context, d *engine.Daemon, in ReserveIn) (any, error) {
			return d.Release(ctx, in.VM, in.GuestPort)
		}),
		own(d, GuestExec, func(ctx context.Context, d *engine.Daemon, in ExecIn) (any, error) {
			ran, err := d.GuestExec(ctx, in.VM, in.Line, in.Stdin)
			return ExecOut{Code: ran.ExitCode, Stdout: ran.Stdout, Stderr: ran.Stderr}, err
		}),
		own(d, Accounts, func(ctx context.Context, d *engine.Daemon, _ struct{}) (any, error) { return d.Accounts(ctx), nil }),
		own(d, StopAll, func(ctx context.Context, d *engine.Daemon, _ struct{}) (any, error) { return d.StopAll(ctx), nil }),
		own(d, Power, func(_ context.Context, d *engine.Daemon, _ struct{}) (any, error) { return d.Power(), nil }),
		own(d, Awake, func(_ context.Context, d *engine.Daemon, in engine.AwakeIn) (any, error) { return d.Awake(in) }),
		own(d, Sleep, func(_ context.Context, d *engine.Daemon, _ struct{}) (any, error) { return d.Sleep(), nil }),
		own(d, Refresh, func(_ context.Context, d *engine.Daemon, in PagesIn) (any, error) {
			return struct{}{}, d.Refresh(in.Pages)
		}),
	)
	for _, verb := range engine.VMVerbs() {
		out = append(out, own(d, VM(verb), func(ctx context.Context, d *engine.Daemon, in VMIn) (any, error) {
			return nil, d.VM(ctx, verb, in.VM)
		}))
	}
	return out
}

type Host struct {
	// value: the registry the engine serves to its clients (the menu bar's
	// actions), run on this Mac; nil serves the engine's own calls alone.
	Registry *call.Registry
	Here     machine.Profile
	Word     func(e *fail.Error) (message, hint string)
	Sleep    func(ctx context.Context, d time.Duration) error
}

func Server(d *engine.Daemon, h Host) serve.Server {
	// invariant: the engine's server: its own calls first, then the registry's,
	// and the watch of its pages; a call of neither is unknown.
	var served []string
	if h.Registry != nil {
		for _, m := range h.Registry.Metas() {
			served = append(served, m.Name)
		}
	}
	byName := map[string]call.Handler{}
	for _, c := range calls(d, served) {
		byName[c.Meta().Name] = c
	}
	return serve.Server{
		Calls: func(name string) (call.Handler, bool) {
			if c, ok := byName[name]; ok {
				return c, true
			}
			if h.Registry == nil {
				return nil, false
			}
			return h.Registry.Lookup(name)
		},
		Streams: map[string]serve.Feed{Page: func(ctx context.Context, raw json.RawMessage, send func(any) error) error {
			var in PagesIn
			if len(raw) > 0 {
				if err := json.Unmarshal(raw, &in); err != nil {
					return err
				}
			}
			return worded(d.Stream(ctx, in.Pages, send))
		}},
		Here: h.Here, Word: h.Word, Hint: "station machines", Sleep: h.Sleep,
	}
}
