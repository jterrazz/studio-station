package calls

import (
	"context"

	"github.com/jterrazz/studio-station/src/core/call"
	"github.com/jterrazz/studio-station/src/core/machine"
	"github.com/jterrazz/studio-station/src/ports"
)

type TailnetIn struct{}

var Tailnet = call.Call[Deps, TailnetIn, ports.Status]{
	Name: "tailnet.read", Where: call.AtTarget, Applies: machine.Anywhere,
	Run: func(ctx context.Context, d Deps, _ TailnetIn, _ call.Progress) (ports.Status, error) {
		return d.Tailnet.Status(ctx)
	},
}
