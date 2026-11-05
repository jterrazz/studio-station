package feeds

import (
	"context"
	"time"

	activitycalls "github.com/jterrazz/studio-station/src/activity/calls"
	"github.com/jterrazz/studio-station/src/core/call"
	"github.com/jterrazz/studio-station/src/core/machine"
	"github.com/jterrazz/studio-station/src/engine"
	"github.com/jterrazz/studio-station/src/page"
	pageactivity "github.com/jterrazz/studio-station/src/pages/activity"
	pageawake "github.com/jterrazz/studio-station/src/pages/awake"
	pagemenubar "github.com/jterrazz/studio-station/src/pages/menubar"
	portscalls "github.com/jterrazz/studio-station/src/ports/calls"
)

func (o Menubar) Ports() engine.Feed {
	// invariant: SPEC §10.2 Ports: the ports of the Mac the page is of, asked
	// through its engine, or of every Mac when it names none; read when it
	// opens, every 30 s and on a change of this engine's state.
	return func(ctx context.Context, in engine.FeedInput) <-chan any {
		read := func(ctx context.Context) page.Page {
			target, of, list := machine.Ref{}, pagemenubar.PortsOf{Machine: in.Subject}, portscalls.ListIn{AllMachines: true}
			if in.Subject != "" {
				target, list = machine.Ref{Name: in.Subject}, portscalls.ListIn{}
			}
			l, err := call.Invoke(ctx, o.Calls, target, portscalls.List, list, nil)
			if err != nil {
				return failed(pagemenubar.PortsID, o.Here, err)
			}
			return pagemenubar.Ports(l, of, o.Here)
		}
		return o.reread(ctx, in, pagemenubar.PortsID, remoteEvery, read)
	}
}

func (o Menubar) Awake() engine.Feed {
	// invariant: SPEC §10.2 Awake: This Mac's Awake then each other Mac's,
	// each through its engine, read when it opens, every 30 s and on a change
	// of this engine's state (a keep-awake taken or let go).
	return func(ctx context.Context, in engine.FeedInput) <-chan any {
		read := func(ctx context.Context) page.Page {
			f, err := o.EveryMac(ctx)
			if err != nil {
				return failed(pagemenubar.AwakeID, o.Here, err)
			}
			return pagemenubar.Awake(f, o.Here)
		}
		return o.reread(ctx, in, pagemenubar.AwakeID, remoteEvery, read)
	}
}

func (o Menubar) Activity() engine.Feed {
	// invariant: SPEC §10.2 Activity, §6.3: This Mac's measures over the
	// window the page names after its colon (1 h unless named), read from the
	// activity watch every 2 s while the page is open.
	return func(ctx context.Context, in engine.FeedInput) <-chan any {
		window := pagemenubar.Window(in.Subject)
		read := func(ctx context.Context) page.Page {
			v := pageactivity.View{Name: o.Here.Machine, Label: "This Mac", Kind: string(machine.Mac), ThisMac: true, Graph: window}
			snaps, err := call.Follow(ctx, o.Calls, machine.Ref{}, activitycalls.Read, activitycalls.ReadIn{Graph: window}, nil)
			if err != nil {
				return failed(pagemenubar.ActivityID, o.Here, err)
			}
			for u := range snaps {
				if u.Err != nil {
					return failed(pagemenubar.ActivityID, o.Here, u.Err)
				}
				v.Report = u.Value
			}
			return pagemenubar.Activity(v, o.Here)
		}
		return o.reread(ctx, in, pagemenubar.ActivityID, sampleEvery, read)
	}
}

// invariant: a page of the menu bar's own goes out at once as its frame,
// loading, then once per read: on its period, on a refresh and on a change
// of the engine's state; nothing of it runs once the watch ends.
func (o Menubar) reread(ctx context.Context, in engine.FeedInput, id page.ID, every time.Duration, read func(context.Context) page.Page) <-chan any {
	out := make(chan any, 1)
	go func() {
		defer close(out)
		out <- page.Document{Schema: page.Schema, Page: page.Page{ID: id, Context: o.Here, State: page.Loading}}
		for {
			p := read(ctx)
			if ctx.Err() != nil {
				return
			}
			select {
			case <-out:
			default:
			}
			select {
			case out <- page.Document{Schema: page.Schema, Page: p}:
			case <-ctx.Done():
				return
			}
			wait, cancel := context.WithCancel(ctx)
			go func() {
				defer cancel()
				select {
				case <-in.Changed:
				case <-in.Refresh:
				case <-wait.Done():
				}
			}()
			_ = o.Clock.Sleep(wait, every)
			cancel()
			if ctx.Err() != nil {
				return
			}
		}
	}()
	return out
}

func failed(id page.ID, here page.Context, err error) page.Page {
	return page.Seal(page.Page{ID: id, Context: here, Error: &page.Error{Message: err.Error()}})
}

// value: the Awake of every Mac, This Mac's first, as the CLI's station
// awake reads it.
type EveryMac func(ctx context.Context) (pageawake.Fleet, error)
