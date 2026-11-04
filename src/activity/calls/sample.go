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

var Sample = call.Call[Deps, SampleIn, activity.Reading]{
	Name: "activity.sample", Section: section.Activity, Where: call.AtTarget, Applies: activity.Applies,
	Run: func(ctx context.Context, d Deps, _ SampleIn, _ call.Progress) (activity.Reading, error) {
		return Read(ctx, d.Meter)
	},
}

func Read(ctx context.Context, m contract.Meter) (activity.Reading, error) {
	// value: the machine's counters and its busiest processes, read together.
	s, err := m.Sample(ctx)
	if err != nil {
		return activity.Reading{}, err
	}
	top, err := m.Top(ctx)
	if err != nil {
		return activity.Reading{}, err
	}
	return activity.Reading{Sample: s, Top: top}, nil
}
