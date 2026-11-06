package engine

import (
	"context"
	"errors"
	"io/fs"
	"strconv"
	"strings"
	"time"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/domain/machines"
	"github.com/jterrazz/studio-station/src/domain/power"
	"github.com/jterrazz/studio-station/src/system"
)

const JobBudget = startWait

const (
	bootoutWait = 10 * time.Second
	startWait   = 15 * time.Second
	controlPoll = 200 * time.Millisecond
)

type ControlMachines interface {
	Records() ([]machines.VMRecord, error)
	StopDirect(ctx context.Context, name string) error
}

type Job interface {
	Loaded(ctx context.Context) bool
	Load(ctx context.Context) error
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
}

type ControlOptions struct {
	// value: this Mac's power, whose clamshell a stop resets; nil is none.
	Power    power.Port
	HereName string
	State    string
	// value: this station's version, which a reading compares with the engine's.
	Version string
	// value: where a stop the owner asks is kept, so a reading names it.
	Marks StopMarks
}

type Control struct {
	// value: station engine stop and start: the one engine of this Mac, its
	// launchd agent and the VMs it supervises.
	h   system.Host
	c   Peer
	m   ControlMachines
	hv  Hypervisor
	job Job
	o   ControlOptions
}

func NewControl(h system.Host, c Peer, m ControlMachines, hv Hypervisor, job Job, o ControlOptions) *Control {
	return &Control{h: h, c: c, m: m, hv: hv, job: job, o: o}
}

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

func (c *Control) Check() error {
	if !c.installed() {
		return &NotInstalledError{}
	}
	return nil
}

func (c *Control) installed() bool {
	_, err := c.h.FS.Stat(catalog.EnginePlist(c.h.Env.Home()))
	return !errors.Is(err, fs.ErrNotExist)
}

func (c *Control) Read(ctx context.Context) (Reading, error) {
	// invariant: a reading never fails for the engine's state: an engine that
	// does not answer is stopped (its agent kept) or not installed.
	r := Reading{Station: c.o.Version}
	hello, err := c.c.Hello(ctx)
	if err != nil {
		r.State = EngineNotInstalled
		if c.installed() {
			r.State = EngineStopped
			if c.o.Marks != nil {
				r.Stopped, _ = c.o.Marks.Read()
			}
		}
		return r, nil
	}
	r.State, r.Version = EngineRunning, hello.Version
	if !hello.StartedAt.IsZero() {
		r.Up = c.h.Clock.Now().Sub(hello.StartedAt)
	}
	if views, err := c.c.VMs(ctx); err == nil {
		for _, v := range views {
			if v.State == StateRunning {
				r.VMs++
			}
		}
	}
	if v, err := c.c.AwakeView(ctx); err == nil && v.Session != nil {
		until := v.Session.Until
		r.AwakeUntil = &until
	}
	return r, nil
}

type Work struct {
	Answering bool
	VMs       []Stopped
	// value: the servers that run: a stop skips them, and the engine keeps
	// running for them (SPEC §5.11).
	Servers []string
}

func (w Work) Empty() bool { return len(w.VMs) == 0 }

func (c *Control) Plan(ctx context.Context) (Work, error) {
	// invariant: what a stop ends is every VM that runs, whatever its lifetime:
	// a stop keeps every disk, and only a delete destroys.
	records, err := c.m.Records()
	if err != nil {
		return Work{}, err
	}
	var w Work
	running := map[string]bool{}
	if views, err := c.c.VMs(ctx); err == nil {
		w.Answering = true
		for _, v := range views {
			running[v.Name] = v.State == StateRunning
		}
	} else {
		for _, r := range records {
			_, running[r.Name] = c.hv.Adopt(ctx, r.Name)
		}
	}
	for _, r := range records {
		switch {
		case !running[r.Name]:
		case r.Lifetime == machines.AlwaysOn:
			w.Servers = append(w.Servers, r.Name)
		default:
			w.VMs = append(w.VMs, Stopped{Name: r.Name, Lifetime: string(r.Lifetime), Result: ResultStopped})
		}
	}
	return w, nil
}

type StopReport struct {
	VMs []Stopped
	// value: "ended" (the engine ended the Awake session), "reset" (the CLI
	// set a dead engine's clamshell back to normal), or "" (nothing held, or
	// no keep-awake here).
	Awake string
	// value: the engine stopped sharing the ports of the VMs it stopped.
	Sharing bool
	// value: "stopped" (booted out now), "already stopped", "kept" (a VM
	// would not stop, so the engine still supervises it), or "for servers"
	// (a server runs, which the engine keeps running).
	Engine string
	// value: the servers it skipped.
	Servers []string
}

const (
	AwakeEnded    = "ended"
	AwakeReleased = "released"
)

const (
	EngineHalted  = "stopped"
	EngineAlready = "already stopped"
	EngineKept    = "kept"
	EngineServers = "for servers"
)

func (c *Control) Stop(ctx context.Context, w Work, by string) (StopReport, error) {
	r := StopReport{Servers: w.Servers}
	if w.Answering {
		all, err := c.c.StopAll(ctx)
		if err != nil {
			return r, err
		}
		r.VMs, r.Sharing = all.VMs, all.SharingEnded
		if all.AwakeEnded {
			r.Awake = AwakeEnded
		}
	} else {
		r.VMs = c.stopDirect(ctx, w)
		// invariant: a clamshell mask a dead engine left is released here, by the
		// same call the engine makes, so a stop never leaves the lid held.
		if c.o.Power != nil && c.o.Power.Clamshell(false) == nil {
			r.Awake = AwakeReleased
		}
	}
	for _, v := range r.VMs {
		if v.Result == ResultFailed {
			r.Engine = EngineKept
			return r, &StuckError{VM: v.Name}
		}
	}
	if len(w.Servers) > 0 {
		r.Engine = EngineServers
		return r, nil
	}
	if !w.Answering && !c.job.Loaded(ctx) {
		r.Engine = EngineAlready
		return r, nil
	}
	if err := c.job.Stop(ctx); err != nil {
		return r, err
	}
	for waited := time.Duration(0); waited < bootoutWait && c.answers(ctx); waited += controlPoll {
		if err := c.h.Clock.Sleep(ctx, controlPoll); err != nil {
			return r, err
		}
	}
	r.Engine = EngineHalted
	if c.o.Marks != nil {
		_ = c.o.Marks.Write(StopMark{At: c.h.Clock.Now().UTC().Truncate(time.Second), By: by})
	}
	return r, nil
}

// invariant: with the engine down, a VM whose run outlived it is ended here,
// by the same path an engine-less stop takes, so nothing station started is
// left running once stop returns.
func (c *Control) stopDirect(ctx context.Context, w Work) []Stopped {
	out := make([]Stopped, 0, len(w.VMs))
	for _, v := range w.VMs {
		if err := c.m.StopDirect(ctx, v.Name); err != nil {
			v.Result, v.Detail = ResultFailed, err.Error()
		}
		out = append(out, v)
	}
	return out
}

type StartReport struct {
	Already bool
	Version string
	PID     int
}

type StartFailedError struct {
	// value: the last line the engine's log holds, or the job manager's
	// answer when the log holds none.
	Last  string
	Cause error
}

func (e *StartFailedError) Error() string { return "the engine did not start" }

func (e *StartFailedError) Unwrap() error { return e.Cause }

func (c *Control) Start(ctx context.Context) (StartReport, error) {
	if p, err := c.c.Hello(ctx); err == nil {
		return StartReport{Already: true, Version: p.Version, PID: p.PID}, nil
	}
	if !c.job.Loaded(ctx) {
		if err := c.job.Load(ctx); err != nil {
			return StartReport{}, c.failed(err)
		}
	}
	if err := c.job.Start(ctx); err != nil {
		return StartReport{}, c.failed(err)
	}
	for waited := time.Duration(0); waited < startWait; waited += controlPoll {
		if p, err := c.c.Hello(ctx); err == nil {
			return StartReport{Version: p.Version, PID: p.PID}, nil
		}
		if err := c.h.Clock.Sleep(ctx, controlPoll); err != nil {
			return StartReport{}, err
		}
	}
	return StartReport{}, c.failed(errors.New("it did not answer in " + strconv.Itoa(int(startWait.Seconds())) + "s"))
}

func (c *Control) failed(cause error) error {
	last := cause.Error()
	if data, err := c.h.FS.ReadFile(LogPath(c.o.State)); err == nil {
		lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
		if line := strings.TrimSpace(lines[len(lines)-1]); line != "" {
			last = line
		}
	}
	return &StartFailedError{Last: last, Cause: cause}
}

func (c *Control) answers(ctx context.Context) bool {
	_, err := c.c.Hello(ctx)
	return err == nil
}
