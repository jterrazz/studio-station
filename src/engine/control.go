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
	Remove(ctx context.Context, name string) (machines.DeleteReport, error)
}

type Job interface {
	Loaded(ctx context.Context) bool
	Load(ctx context.Context) error
	Restart(ctx context.Context) error
	Stop(ctx context.Context) error
}

type ControlOptions struct {
	Here     catalog.MachineKind
	HereName string
	State    string
}

type Control struct {
	// value: station stop and station start: the one daemon of this host, its
	// launchd agent and the VMs it supervises.
	h   system.Host
	c   Client
	m   ControlMachines
	hv  Hypervisor
	job Job
	o   ControlOptions
}

func NewControl(h system.Host, c Client, m ControlMachines, hv Hypervisor, job Job, o ControlOptions) *Control {
	return &Control{h: h, c: c, m: m, hv: hv, job: job, o: o}
}

type NotInstalledError struct{}

func (*NotInstalledError) Error() string { return "the station daemon is not installed" }

type OffHostError struct {
	Verb string
	Here catalog.MachineKind
}

func (e *OffHostError) Error() string { return "station " + e.Verb + " runs on a host; here is a VM" }

type UnconfirmedError struct{ Deletes, Stops []string }

func (e *UnconfirmedError) Error() string {
	var parts []string
	if len(e.Deletes) > 0 {
		parts = append(parts, "deletes "+strings.Join(e.Deletes, ", "))
	}
	if len(e.Stops) > 0 {
		parts = append(parts, "stops "+strings.Join(e.Stops, ", "))
	}
	return "station stop " + strings.Join(parts, " and ")
}

type StuckError struct{ VM string }

func (e *StuckError) Error() string { return e.VM + " did not stop; the daemon keeps running" }

type NoAnswerError struct{ Wait time.Duration }

func (e *NoAnswerError) Error() string {
	return "the station daemon did not answer in " + strconv.Itoa(int(e.Wait.Seconds())) + "s"
}

type BootError struct {
	Message string
	// value: the command line the owner types to read the daemon's job.
	See   string
	Cause error
}

func (e *BootError) Error() string { return e.Message }

func (e *BootError) Unwrap() error { return e.Cause }

func (c *Control) Check(verb string) error {
	if c.o.Here != catalog.Host {
		return &OffHostError{Verb: verb, Here: c.o.Here}
	}
	if _, err := c.h.FS.Stat(c.plist()); errors.Is(err, fs.ErrNotExist) {
		return &NotInstalledError{}
	}
	return nil
}

func (c *Control) plist() string { return catalog.DaemonPlist(c.h.Env.Home()) }

type Work struct {
	Answering bool
	VMs       []Stopped
}

func (w Work) Empty() bool { return len(w.VMs) == 0 }

func (c *Control) Plan(ctx context.Context) (Work, error) {
	records, err := c.m.Records()
	if err != nil {
		return Work{}, err
	}
	var w Work
	running := map[string]bool{}
	var views []VMView
	if err := c.c.Do(ctx, "vms", Request{}, &views); err == nil {
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
		if !running[r.Name] {
			continue
		}
		s := Stopped{Name: r.Name, Lifetime: string(r.Lifetime), Result: ResultStopped}
		if r.Lifetime == machines.Temporary {
			s.Result = ResultDeleted
		}
		w.VMs = append(w.VMs, s)
	}
	return w, nil
}

type StopReport struct {
	VMs []Stopped
	// value: "ended" (the daemon ended a session), "reset" (the CLI set a dead
	// daemon's clamshell back to normal), or "" (nothing held, or no Power here).
	Power string
	// value: "stopped" (booted out now), "already stopped", or "kept" (a VM
	// would not stop, so the daemon still supervises it).
	Daemon string
}

const (
	PowerEnded = "ended"
	PowerReset = "reset"
)

const (
	DaemonStopped = "stopped"
	DaemonAlready = "already stopped"
	DaemonKept    = "kept"
)

func (c *Control) Stop(ctx context.Context, w Work) (StopReport, error) {
	var r StopReport
	if w.Answering {
		var all StopAll
		if err := c.c.Do(ctx, "stop_all", Request{}, &all); err != nil {
			return r, err
		}
		r.VMs = all.VMs
		if all.PowerEnded {
			r.Power = PowerEnded
		}
	} else {
		r.VMs = c.stopDirect(ctx, w)
		// invariant: a clamshell mask a dead daemon left is reset here, by the
		// same call the daemon makes, so a stop never leaves the lid held.
		if c.h.Power != nil && c.h.Power.SetClamshellSleep(false) == nil {
			r.Power = PowerReset
		}
	}
	for _, v := range r.VMs {
		if v.Result == ResultFailed {
			r.Daemon = DaemonKept
			return r, &StuckError{VM: v.Name}
		}
	}
	if !w.Answering && !c.job.Loaded(ctx) {
		r.Daemon = DaemonAlready
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
	r.Daemon = DaemonStopped
	return r, nil
}

// invariant: with the daemon down, a VM whose run outlived it is ended here,
// by the same path a daemon-less stop and delete take, so nothing station
// started is left running once stop returns.
func (c *Control) stopDirect(ctx context.Context, w Work) []Stopped {
	out := make([]Stopped, 0, len(w.VMs))
	for _, v := range w.VMs {
		var err error
		if v.Result == ResultDeleted {
			_, err = c.m.Remove(ctx, v.Name)
		} else {
			err = c.m.StopDirect(ctx, v.Name)
		}
		if err != nil {
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
	Back    []string
}

func (c *Control) Start(ctx context.Context) (StartReport, error) {
	if p, err := c.c.Ping(ctx); err == nil {
		return StartReport{Already: true, Version: p.Version, PID: p.PID}, nil
	}
	if c.job.Loaded(ctx) {
		if err := c.job.Restart(ctx); err != nil {
			return StartReport{}, err
		}
	} else if err := c.job.Load(ctx); err != nil {
		return StartReport{}, err
	}
	for waited := time.Duration(0); waited < startWait; waited += controlPoll {
		if p, err := c.c.Ping(ctx); err == nil {
			r := StartReport{Version: p.Version, PID: p.PID}
			r.Back = c.back(ctx)
			return r, nil
		}
		if err := c.h.Clock.Sleep(ctx, controlPoll); err != nil {
			return StartReport{}, err
		}
	}
	return StartReport{}, &NoAnswerError{Wait: startWait}
}

func (c *Control) back(ctx context.Context) []string {
	records, _ := c.m.Records()
	var views []VMView
	_ = c.c.Do(ctx, "vms", Request{}, &views)
	var out []string
	for _, r := range records {
		for _, v := range views {
			if v.Name == r.Name && r.Lifetime == machines.Lasting && v.State == StateRunning {
				out = append(out, r.Name)
			}
		}
	}
	return out
}

func (c *Control) answers(ctx context.Context) bool {
	_, err := c.c.Ping(ctx)
	return err == nil
}
