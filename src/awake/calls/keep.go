package calls

import (
	"context"

	"github.com/jterrazz/studio-station/src/awake"
	"github.com/jterrazz/studio-station/src/core/call"
	"github.com/jterrazz/studio-station/src/core/machine"
)

var Keep = call.Call[Deps, awake.Request, awake.Awoken]{
	Name: "power.awake", Where: call.InEngine, Applies: machine.Anywhere,
	Run: func(ctx context.Context, d Deps, in awake.Request, _ call.Progress) (awake.Awoken, error) {
		if err := d.Power.CheckFor(in.For); in.For != 0 && err != nil {
			return awake.Awoken{}, err
		}
		if err := d.Power.Check(); err != nil {
			return awake.Awoken{}, err
		}
		return d.Power.Awake(ctx, in)
	},
}
