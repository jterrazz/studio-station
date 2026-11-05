package feeds

import (
	"context"
	"sync"
	"time"

	accountscalls "github.com/jterrazz/studio-station/src/accounts/calls"
	activitycalls "github.com/jterrazz/studio-station/src/activity/calls"
	"github.com/jterrazz/studio-station/src/core/call"
	"github.com/jterrazz/studio-station/src/core/confirm"
	"github.com/jterrazz/studio-station/src/core/machine"
	"github.com/jterrazz/studio-station/src/engine"
	machinescalls "github.com/jterrazz/studio-station/src/machines/calls"
	overviewcalls "github.com/jterrazz/studio-station/src/overview/calls"
	"github.com/jterrazz/studio-station/src/page"
	pagemenubar "github.com/jterrazz/studio-station/src/pages/menubar"
	portscalls "github.com/jterrazz/studio-station/src/ports/calls"
)

func (o Menubar) Machine() engine.Feed {
	// invariant: SPEC §10.3: a machine's page is read only while it is open:
	// its activity every 2 s, the rest every 30 s and again as soon as the
	// engine's state moves or a refresh names the page, each through the
	// router, so another Mac's page is that Mac's engine's answer; the channel
	// closes when the watch ends.
	return func(ctx context.Context, in engine.FeedInput) <-chan any {
		out := make(chan any, 1)
		m := &one{o: o, out: out, m: pagemenubar.Machine{Name: in.Subject, Asks: levels}}
		go func() {
			defer close(out)
			m.send(ctx, func(*pagemenubar.Machine) {})
			var wg sync.WaitGroup
			for _, loop := range []func(context.Context, engine.FeedInput){m.sample, m.reads} {
				wg.Add(1)
				go func() {
					defer wg.Done()
					loop(ctx, in)
				}()
			}
			wg.Wait()
		}()
		return out
	}
}

func (o Menubar) Switcher() engine.Feed {
	// invariant: the switcher is every Mac and its machines as a tree, read
	// when it opens, every 30 s while it stays open, and again as soon as the
	// engine's state moves or a refresh names it.
	return func(ctx context.Context, in engine.FeedInput) <-chan any {
		out := make(chan any, 1)
		go func() {
			defer close(out)
			for {
				f := &menubar{o: o}
				fl, sv, ferr, serr := f.read(ctx, false)
				p := pagemenubar.Switcher(nil, nil)
				if ferr == nil {
					svc := &sv
					if serr != nil {
						svc = nil
					}
					p = pagemenubar.Switcher(&fl, svc)
				}
				latest(ctx, out, page.Document{Schema: page.Schema, Page: p})
				if rest(ctx, o.Clock, remoteEvery, in) != nil {
					return
				}
			}
		}()
		return out
	}
}

// value: the wait between two reads of a page: its period, cut short by the
// engine's state moving or by a refresh; an error once the watch ends.
func rest(ctx context.Context, clock Clock, every time.Duration, in engine.FeedInput) error {
	wait, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		select {
		case <-in.Changed:
		case <-in.Refresh:
		case <-wait.Done():
		}
		cancel()
	}()
	_ = clock.Sleep(wait, every)
	return ctx.Err()
}

func latest(ctx context.Context, out chan any, p any) {
	select {
	case <-out:
	default:
	}
	select {
	case out <- p:
	case <-ctx.Done():
	}
}

// value: the confirmation rule of the calls, which the page only draws.
func levels(gesture string, kind machine.Kind) string {
	return string(confirm.For(confirm.Action(gesture), kind))
}

type one struct {
	o   Menubar
	mu  sync.Mutex
	m   pagemenubar.Machine
	out chan any
}

func (f *one) send(ctx context.Context, change func(*pagemenubar.Machine)) {
	f.mu.Lock()
	change(&f.m)
	f.m.Now = f.o.Clock.Now()
	p := page.Document{Schema: page.Schema, Page: pagemenubar.MachinePage(f.m)}
	f.mu.Unlock()
	latest(ctx, f.out, p)
}

func (f *one) sample(ctx context.Context, _ engine.FeedInput) {
	at := machine.Ref{Name: f.m.Name}
	for {
		if s, err := call.Invoke(ctx, f.o.Calls, at, activitycalls.Sample, activitycalls.SampleIn{}, nil); err == nil {
			f.send(ctx, func(m *pagemenubar.Machine) {
				m.Samples = append(m.Samples, s)
				if len(m.Samples) > sampleKept {
					m.Samples = m.Samples[len(m.Samples)-sampleKept:]
				}
			})
		}
		if f.o.Clock.Sleep(ctx, sampleEvery) != nil {
			return
		}
	}
}

func (f *one) reads(ctx context.Context, in engine.FeedInput) {
	at := machine.Ref{Name: f.m.Name}
	for {
		listed, ferr := call.Read(ctx, f.o.Calls, machine.Ref{}, machinescalls.List, machinescalls.ListIn{Unwritten: true}, nil)
		shared, serr := call.Invoke(ctx, f.o.Calls, machine.Ref{}, portscalls.List, portscalls.ListIn{AllMachines: true}, nil)
		lent, aerr := call.Read(ctx, f.o.Calls, machine.Ref{}, accountscalls.List, accountscalls.ListIn{VMs: true}, nil)
		read, oerr := call.Read(ctx, f.o.Calls, at, overviewcalls.Read, overviewcalls.ReadIn{}, nil)
		f.send(ctx, func(m *pagemenubar.Machine) {
			if ferr == nil {
				m.Fleet = &listed.Fleet
			}
			if serr == nil {
				m.Services = &shared.Fleet
			}
			if aerr == nil {
				m.Accounts = &lent
			}
			if oerr == nil {
				m.Summary = &read.Summary
			}
		})
		if rest(ctx, f.o.Clock, remoteEvery, in) != nil {
			return
		}
	}
}
