package calls

import (
	"context"

	"github.com/jterrazz/studio-station/src/awake"
	"github.com/jterrazz/studio-station/src/core/call"
)

type OffIn struct{}

var Off = call.Call[Deps, OffIn, awake.Slept]{
	Name: "awake.off", Where: call.InEngine, Applies: awake.Applies,
	Run: func(ctx context.Context, d Deps, _ OffIn, _ call.Progress) (awake.Slept, error) {
		// invariant: off ends the keep-awake only: Never sleeps is a
		// setting, turned off as one.
		if err := d.Power.Check(); err != nil {
			return awake.Slept{}, err
		}
		s, err := d.Power.Sleep(ctx)
		s.Mac, s.NeverSleeps = d.Mac, d.never()
		return s, err
	},
}
