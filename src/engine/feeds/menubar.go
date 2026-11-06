package feeds

import (
	"context"
	"sync"
	"time"

	activitycalls "github.com/jterrazz/studio-station/src/activity/calls"
	"github.com/jterrazz/studio-station/src/awake"
	awakecalls "github.com/jterrazz/studio-station/src/awake/calls"
	"github.com/jterrazz/studio-station/src/core/call"
	"github.com/jterrazz/studio-station/src/core/machine"
	"github.com/jterrazz/studio-station/src/engine"
	"github.com/jterrazz/studio-station/src/machines"
	machinescalls "github.com/jterrazz/studio-station/src/machines/calls"
	overviewcalls "github.com/jterrazz/studio-station/src/overview/calls"
	"github.com/jterrazz/studio-station/src/page"
	pagemenubar "github.com/jterrazz/studio-station/src/pages/menubar"
	"github.com/jterrazz/studio-station/src/ports"
	portscalls "github.com/jterrazz/studio-station/src/ports/calls"
)

const (
	sampleEvery = 2 * time.Second
	remoteEvery = 30 * time.Second
	todoEvery   = 15 * time.Minute
	sampleKept  = 16
)

type Menubar struct {
	// value: the router the feed reads this Mac through, its calls run in
	// the engine's process.
	Calls       call.Router
	Clock       Clock
	ThisMachine page.Context
	// value: the Power report of a view the engine holds.
	Report func(awake.View) *awake.Report
	// value: every Mac's Awake, This Mac's first, as the CLI's station awake
	// reads it, for the Awake page.
	EveryMac EveryMac
}

type Clock interface {
	Sleep(ctx context.Context, d time.Duration) error
	Now() time.Time
}

type menubar struct {
	o   Menubar
	mu  sync.Mutex
	r   pagemenubar.Reading
	out chan any
}

func (o Menubar) Feed() engine.Feed {
	return func(ctx context.Context, in engine.FeedInput) <-chan any {
		f := &menubar{o: o, out: make(chan any, 1)}
		go f.run(ctx, in)
		return f.out
	}
}

// invariant: the frame goes out at once, then a page each time one of the
// reads answers; every read runs only while this feed does, so nothing of it
// runs at rest, and the channel closes when the watch ends.
func (f *menubar) run(ctx context.Context, in engine.FeedInput) {
	defer close(f.out)
	f.update(ctx, func(r *pagemenubar.Reading) { r.Power = f.o.Report(in.Power()) })
	var wg sync.WaitGroup
	for _, loop := range []func(context.Context, engine.FeedInput){f.sample, f.todo, f.remote, f.local} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			loop(ctx, in)
		}()
	}
	wg.Wait()
}

func (f *menubar) update(ctx context.Context, change func(*pagemenubar.Reading)) {
	f.mu.Lock()
	change(&f.r)
	p := page.Document{Schema: page.Schema, Page: pagemenubar.Page(f.r, f.o.ThisMachine)}
	f.mu.Unlock()
	select {
	case <-f.out:
	default:
	}
	select {
	case f.out <- p:
	case <-ctx.Done():
	}
}

func (f *menubar) sample(ctx context.Context, _ engine.FeedInput) {
	for {
		if s, err := call.Invoke(ctx, f.o.Calls, machine.Ref{}, activitycalls.Sample, activitycalls.SampleIn{}, nil); err == nil {
			f.update(ctx, func(r *pagemenubar.Reading) {
				r.Samples = append(r.Samples, s)
				if len(r.Samples) > sampleKept {
					r.Samples = r.Samples[len(r.Samples)-sampleKept:]
				}
			})
		}
		if f.o.Clock.Sleep(ctx, sampleEvery) != nil {
			return
		}
	}
}

// invariant: the To do is read whole at the feed's start, then again from
// fresh readings every 15 min or on a refresh.
func (f *menubar) todo(ctx context.Context, in engine.FeedInput) {
	fresh := false
	for {
		if snaps, err := call.Follow(ctx, f.o.Calls, machine.Ref{}, overviewcalls.Read, overviewcalls.ReadIn{Fresh: fresh}, nil); err == nil {
			for u := range snaps {
				if u.Err != nil {
					continue
				}
				summary := u.Value.Summary
				f.update(ctx, func(r *pagemenubar.Reading) { r.Home = &summary })
			}
		}
		wait, cancel := context.WithCancel(ctx)
		go func() {
			select {
			case <-in.Refresh:
			case <-wait.Done():
			}
			cancel()
		}()
		_ = f.o.Clock.Sleep(wait, todoEvery)
		cancel()
		if ctx.Err() != nil {
			return
		}
		fresh = true
	}
}

func (f *menubar) read(ctx context.Context, thisMac bool) (allMachines machines.AllMachines, portsRead ports.AllMachines, allMachinesErr, portsErr error) {
	listed, allMachinesErr := call.Read(ctx, f.o.Calls, machine.Ref{}, machinescalls.List, machinescalls.ListIn{ThisMac: thisMac, Unwritten: true}, nil)
	shared, portsErr := call.Invoke(ctx, f.o.Calls, machine.Ref{}, portscalls.List, portscalls.ListIn{AllMachines: !thisMac}, nil)
	return listed.All, shared.AllMachines, allMachinesErr, portsErr
}

func (f *menubar) remote(ctx context.Context, _ engine.FeedInput) {
	for {
		fl, sv, ferr, serr := f.read(ctx, false)
		macs := f.awake(ctx, fl)
		f.update(ctx, func(r *pagemenubar.Reading) {
			if ferr == nil {
				r.All = &fl
				r.Macs = macs
			}
			if serr == nil {
				r.Ports = &sv
			}
		})
		if f.o.Clock.Sleep(ctx, remoteEvery) != nil {
			return
		}
	}
}

// value: each other Mac's Awake, as its own engine reports it; a Mac that
// does not answer is left out.
func (f *menubar) awake(ctx context.Context, fl machines.AllMachines) []awake.Report {
	var out []awake.Report
	for _, h := range fl.Macs {
		if h.ThisMac || !h.Reachable {
			continue
		}
		if r, err := call.Read(ctx, f.o.Calls, machine.Ref{Name: h.Name}, awakecalls.Read, awakecalls.ReadIn{}, nil); err == nil {
			r.Mac = h.Name
			out = append(out, r)
		}
	}
	return out
}

// invariant: a change of the engine's own state rereads this host only and
// keeps the remote hosts as last read.
func (f *menubar) local(ctx context.Context, in engine.FeedInput) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-in.Changed:
		}
		fl, sv, ferr, serr := f.read(ctx, true)
		f.update(ctx, func(r *pagemenubar.Reading) {
			r.Power = f.o.Report(in.Power())
			if ferr == nil && len(fl.Macs) == 1 {
				r.All = hereAllMachines(r.All, fl)
			}
			if serr == nil && len(sv.Macs) == 1 {
				r.Ports = thisMacPorts(r.Ports, sv)
			}
		})
	}
}

// value: the fleet as last read with this host's row replaced by here's
// one; here itself when nothing was read yet.
func hereAllMachines(was *machines.AllMachines, thisMac machines.AllMachines) *machines.AllMachines {
	if was == nil {
		return &thisMac
	}
	out := *was
	out.Macs = append([]machines.MacView(nil), was.Macs...)
	for i, h := range out.Macs {
		if h.ThisMac {
			out.Macs[i] = thisMac.Macs[0]
		}
	}
	return &out
}

func thisMacPorts(was *ports.AllMachines, thisMac ports.AllMachines) *ports.AllMachines {
	if was == nil {
		return &thisMac
	}
	out := *was
	out.Macs = append([]ports.MacPorts(nil), was.Macs...)
	for i, h := range out.Macs {
		if h.ThisMac {
			out.Macs[i] = thisMac.Macs[0]
		}
	}
	return &out
}
