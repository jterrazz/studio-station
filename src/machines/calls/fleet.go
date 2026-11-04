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
}

type ListOut struct {
	Fleet machines.Fleet
	// value: why the owner's client blocks for the fleet were not rewritten.
	Unrefreshed error
}

var List = call.Call[Deps, ListIn, ListOut]{
	Name: "machines.list", Where: call.InProcess, Applies: machine.Anywhere,
	Run: func(ctx context.Context, d Deps, in ListIn, _ call.Progress) (ListOut, error) {
		fleet, err := d.Machines.Fleet(ctx, in.Here)
		if err != nil {
			return ListOut{}, err
		}
		out := ListOut{Fleet: fleet}
		if !in.Here {
			out.Unrefreshed = d.Machines.SSHBlocks(ctx, fleet)
		}
		return out, nil
	},
}

type CreateIn struct{ Request machines.CreateRequest }

var Create = call.Call[Deps, CreateIn, machines.Created]{
	Name: "machines.create", Where: call.InEngine, Applies: machine.Anywhere,
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
