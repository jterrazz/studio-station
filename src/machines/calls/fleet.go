package calls

import (
	"context"

	"github.com/jterrazz/studio-station/src/core/call"
	"github.com/jterrazz/studio-station/src/core/machine"
	"github.com/jterrazz/studio-station/src/machines"
)

type ListIn struct {
	// value: this host only, without asking the others.
	Here bool
	// value: the fleet read only, the owner's client blocks left as they are.
	Unwritten bool
}

type ListOut struct {
	Fleet machines.Fleet
	// value: why the owner's client blocks for the fleet were not rewritten.
	Unrefreshed error
}

var List = call.Watch[Deps, ListIn, ListOut]{
	// invariant: each update is the fleet so far, a row more as each Mac
	// answers; the last holds every Mac, and the ssh blocks are rewritten
	// for it alone.
	Name: "machines.list", Where: call.InProcess, Applies: machine.Anywhere,
	Run: func(ctx context.Context, d Deps, in ListIn, _ call.Progress) (<-chan call.Update[ListOut], error) {
		updates := make(chan call.Update[ListOut])
		send := func(u call.Update[ListOut]) {
			select {
			case updates <- u:
			case <-ctx.Done():
			}
		}
		go func() {
			defer close(updates)
			fleet, err := d.Machines.FleetAsItFills(ctx, in.Here, func(f machines.Fleet) { send(call.Update[ListOut]{Value: ListOut{Fleet: f}}) })
			if err != nil {
				send(call.Update[ListOut]{Err: err})
				return
			}
			out := ListOut{Fleet: fleet}
			if !in.Here && !in.Unwritten {
				out.Unrefreshed = d.Machines.SSHBlocks(ctx, fleet)
			}
			send(call.Update[ListOut]{Value: out})
		}()
		return updates, nil
	},
}

type CreateIn struct{ Request machines.CreateRequest }

var CreateWorkspace = call.Call[Deps, CreateIn, machines.Created]{
	Name: "machines.create-workspace", Where: call.InEngine, Applies: machines.CanCreate,
	Run: func(ctx context.Context, d Deps, in CreateIn, p call.Progress) (machines.Created, error) {
		last := ""
		created, err := d.Machines.Create(ctx, in.Request, func(s machines.Step) {
			if last != "" {
				p.Done(last, nil)
			}
			last = s.Label
			p.Step(s.Label, "")
		})
		if last != "" {
			p.Done(last, err)
		}
		return created, err
	},
}

type ProbeIn struct{ Name string }

var Probe = call.Call[Deps, ProbeIn, machines.ProbeReport]{
	Name: "machines.probe", Where: call.InProcess, Applies: machine.Anywhere,
	Run: func(ctx context.Context, d Deps, in ProbeIn, _ call.Progress) (machines.ProbeReport, error) {
		if err := d.Machines.CheckRemote(in.Name); err != nil {
			return machines.ProbeReport{}, err
		}
		return d.Machines.Probe(ctx, in.Name)
	},
}
