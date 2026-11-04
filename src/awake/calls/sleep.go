package calls

import (
	"context"

	"github.com/jterrazz/studio-station/src/awake"
	"github.com/jterrazz/studio-station/src/core/call"
	"github.com/jterrazz/studio-station/src/core/machine"
)

type SleepIn struct{}

var Sleep = call.Call[Deps, SleepIn, awake.Slept]{
	Name: "power.sleep", Where: call.InEngine, Applies: machine.Anywhere,
	Run: func(ctx context.Context, d Deps, _ SleepIn, _ call.Progress) (awake.Slept, error) {
		if err := d.Power.Check(); err != nil {
			return awake.Slept{}, err
		}
		return d.Power.Sleep(ctx)
	},
}
