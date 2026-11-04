package calls

import (
	"context"

	"github.com/jterrazz/studio-station/src/activity"
	"github.com/jterrazz/studio-station/src/activity/contract"
	"github.com/jterrazz/studio-station/src/core/call"
	"github.com/jterrazz/studio-station/src/core/section"
)

type Deps struct {
	Meter contract.Meter
}

func All(d Deps) []call.Handler {
	return []call.Handler{call.Bind(Sample, d)}
}

type SampleIn struct{}

var Sample = call.Call[Deps, SampleIn, activity.Sample]{
	Name: "activity.sample", Section: section.Activity, Where: call.AtTarget, Applies: activity.Applies,
	Run: func(ctx context.Context, d Deps, _ SampleIn, _ call.Progress) (activity.Sample, error) {
		return d.Meter.Sample(ctx)
	},
}
