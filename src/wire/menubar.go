package wire

import (
	"context"
	"sync"
	"time"

	"github.com/jterrazz/studio-station/src/domain/machines"
	"github.com/jterrazz/studio-station/src/domain/power"
	"github.com/jterrazz/studio-station/src/domain/services"
	"github.com/jterrazz/studio-station/src/engine"
	overviewcalls "github.com/jterrazz/studio-station/src/overview/calls"
	"github.com/jterrazz/studio-station/src/page"
	pagemenubar "github.com/jterrazz/studio-station/src/pages/menubar"
	"github.com/jterrazz/studio-station/src/surface/cli/kit"
)

const (
	sampleEvery = 2 * time.Second
	remoteEvery = 30 * time.Second
	todoEvery   = 15 * time.Minute
	sampleKept  = 16
)

type menubarFeed struct {
	m      machine
	status *overviewcalls.Overview
	fleet  *machines.Service
	svcs   *services.Service

	mu  sync.Mutex
	r   pagemenubar.Reading
	out chan any
}

func (m machine) menubarFeed(env kit.Env) engine.Feed {
	return func(ctx context.Context, in engine.FeedInput) <-chan any {
		f := &menubarFeed{m: m, status: statusService(env, m), fleet: m.machines(false), svcs: m.services(env), out: make(chan any, 1)}
		go f.run(ctx, in)
		return f.out
	}
}

// invariant: the frame goes out at once, then a page each time one of the
// reads answers; every read runs only while this feed does, so nothing of it
// runs at rest, and the channel closes when the subscription ends.
func (f *menubarFeed) run(ctx context.Context, in engine.FeedInput) {
	defer close(f.out)
	f.update(ctx, func(r *pagemenubar.Reading) { r.Power = f.report(in.Power()) })
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

func (f *menubarFeed) update(ctx context.Context, change func(*pagemenubar.Reading)) {
	f.mu.Lock()
	change(&f.r)
	p := page.Document{Schema: page.Schema, Page: pagemenubar.Page(f.r, f.m.where)}
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

func (f *menubarFeed) report(v power.View) *power.Report {
	return &power.Report{Now: f.m.host.Clock.Now(), Zone: time.Local, Daemon: power.DaemonRunning, Reading: v.Reading, Session: v.Session, LastEnd: v.LastEnd, Guards: v.Guards}
}

func (f *menubarFeed) sample(ctx context.Context, _ engine.FeedInput) {
	for {
		if s, err := f.status.Sample(ctx); err == nil {
			f.update(ctx, func(r *pagemenubar.Reading) {
				r.Samples = append(r.Samples, s)
				if len(r.Samples) > sampleKept {
					r.Samples = r.Samples[len(r.Samples)-sampleKept:]
				}
			})
		}
		if f.m.host.Clock.Sleep(ctx, sampleEvery) != nil {
			return
		}
	}
}

func (f *menubarFeed) todo(ctx context.Context, in engine.FeedInput) {
	for {
		for snap := range f.status.Watch(ctx) {
			f.update(ctx, func(r *pagemenubar.Reading) { r.Home = &snap })
		}
		wait, cancel := context.WithCancel(ctx)
		go func() {
			select {
			case <-in.Refresh:
			case <-wait.Done():
			}
			cancel()
		}()
		_ = f.m.host.Clock.Sleep(wait, todoEvery)
		cancel()
		if ctx.Err() != nil {
			return
		}
		f.status.Forget()
	}
}

func (f *menubarFeed) remote(ctx context.Context, _ engine.FeedInput) {
	for {
		fl, ferr := f.fleet.Fleet(ctx, false)
		sv, serr := f.svcs.List(ctx, false)
		f.update(ctx, func(r *pagemenubar.Reading) {
			if ferr == nil {
				r.Fleet = &fl
			}
			if serr == nil {
				r.Services = &sv
			}
		})
		if f.m.host.Clock.Sleep(ctx, remoteEvery) != nil {
			return
		}
	}
}

// invariant: a change of the daemon's own state rereads this host only and
// keeps the remote hosts as last read.
func (f *menubarFeed) local(ctx context.Context, in engine.FeedInput) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-in.Changed:
		}
		fl, ferr := f.fleet.Fleet(ctx, true)
		sv, serr := f.svcs.List(ctx, true)
		f.update(ctx, func(r *pagemenubar.Reading) {
			r.Power = f.report(in.Power())
			if ferr == nil && len(fl.Hosts) == 1 {
				r.Fleet = hereFleet(r.Fleet, fl.Hosts[0])
			}
			if serr == nil && len(sv.Hosts) == 1 {
				r.Services = hereServices(r.Services, sv.Hosts[0])
			}
		})
	}
}

func hereFleet(was *machines.Fleet, here machines.HostView) *machines.Fleet {
	out := machines.Fleet{Hosts: []machines.HostView{here}}
	if was != nil {
		out = *was
		out.Hosts = append([]machines.HostView(nil), was.Hosts...)
		for i, h := range out.Hosts {
			if h.Here {
				out.Hosts[i] = here
			}
		}
	}
	return &out
}

func hereServices(was *services.Fleet, here services.HostView) *services.Fleet {
	out := services.Fleet{Schema: services.Schema, Hosts: []services.HostView{here}}
	if was != nil {
		out = *was
		out.Hosts = append([]services.HostView(nil), was.Hosts...)
		for i, h := range out.Hosts {
			if h.Here {
				out.Hosts[i] = here
			}
		}
	}
	return &out
}
