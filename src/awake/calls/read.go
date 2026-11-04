package calls

import (
	"context"

	"github.com/jterrazz/studio-station/src/awake"
	"github.com/jterrazz/studio-station/src/core/call"
	"github.com/jterrazz/studio-station/src/core/machine"
)

type ReadIn struct{}

var Read = call.Watch[Deps, ReadIn, awake.Report]{
	Name: "power.read", Where: call.InEngine, Applies: machine.Anywhere,
	Run: func(ctx context.Context, d Deps, _ ReadIn, _ call.Progress) (<-chan call.Update[awake.Report], error) {
		if err := d.Power.Check(); err != nil {
			return nil, err
		}
		return call.Updates(ctx, d.Power.Read(ctx)), nil
	},
}
