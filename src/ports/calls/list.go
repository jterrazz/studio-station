package calls

import (
	"context"

	"github.com/jterrazz/studio-station/src/core/call"
	"github.com/jterrazz/studio-station/src/core/section"
	"github.com/jterrazz/studio-station/src/ports"
)

type ListIn struct {
	// value: every Mac's ports, each asked through its engine.
	AllMachines bool
}

var List = call.Call[Deps, ListIn, ports.Listing]{
	Name: "ports.list", Section: section.Ports, Where: call.AtTarget, Applies: ports.Applies,
	Run: func(ctx context.Context, d Deps, in ListIn, _ call.Progress) (ports.Listing, error) {
		fleet, err := d.Ports.List(ctx, !in.AllMachines)
		if _, inside := d.Ports.Placed(); err != nil || inside {
			return ports.Listing{Fleet: fleet}, err
		}
		node, err := d.Tailnet.Status(ctx)
		return ports.Listing{Fleet: fleet, Tailnet: node}, err
	},
}
