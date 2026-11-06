package calls

import (
	"context"
	"errors"

	"github.com/jterrazz/studio-station/src/core/call"
	"github.com/jterrazz/studio-station/src/core/machine"
	"github.com/jterrazz/studio-station/src/machines"
)

type ListIn struct {
	// value: this host only, without asking the others.
	ThisMac bool
	// value: the fleet read only, the owner's client blocks left as they are.
	Unwritten bool
}

type ListOut struct {
	All machines.AllMachines
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
			allMachines, err := d.Machines.AllMachinesAsItFills(ctx, in.ThisMac, func(f machines.AllMachines) { send(call.Update[ListOut]{Value: ListOut{All: f}}) })
			if err != nil {
				send(call.Update[ListOut]{Err: err})
				return
			}
			out := ListOut{All: allMachines}
			if !in.ThisMac && !in.Unwritten {
				out.Unrefreshed = errors.Join(d.Machines.SSHBlocks(ctx, allMachines), d.Machines.Remember(ctx, allMachines))
			}
			send(call.Update[ListOut]{Value: out})
		}()
		return updates, nil
	},
}

type CreateIn struct{ Request machines.CreateRequest }

var CreateWorkspace = call.Call[Deps, CreateIn, machines.Created]{
	// invariant: a workspace, throwaway or permanent; always on is a server's.
	Name: "machines.create-workspace", Where: call.InEngine, Applies: machines.CanCreate,
	Run: func(ctx context.Context, d Deps, in CreateIn, p call.Progress) (machines.Created, error) {
		r := in.Request
		r.Server = false
		return create(ctx, d, r, p)
	},
}

type CreateServerIn struct {
	Name, Image, Network string
	Resources            machines.Resources
	// value: its health check, a URL or host:port; empty for none.
	Check string
}

var CreateServer = call.Call[Deps, CreateServerIn, machines.Created]{
	// invariant: a server is always on, behind its Mac, and named; station
	// sets up nothing inside it (SPEC §7.1).
	Name: "machines.create-server", Where: call.InEngine, Applies: machines.CanCreate,
	Run: func(ctx context.Context, d Deps, in CreateServerIn, p call.Progress) (machines.Created, error) {
		created, err := create(ctx, d, machines.CreateRequest{Image: in.Image, Name: in.Name, Server: true, Network: in.Network, Resources: in.Resources, Check: in.Check}, p)
		created.OnALaptop = err == nil && d.Laptop
		return created, err
	},
}

func create(ctx context.Context, d Deps, r machines.CreateRequest, p call.Progress) (machines.Created, error) {
	last := ""
	created, err := d.Machines.Create(ctx, r, func(s machines.Step) {
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
