package fake

import (
	"context"
	"slices"
	"time"

	"github.com/jterrazz/studio-station/src/activity"
)

type Meter struct {
	sample  activity.Sample
	up      time.Duration
	battery string
	memory  int64
}

func NewMeter(sample activity.Sample, up time.Duration, battery string, memory int64) *Meter {
	// value: a machine whose counters read sample, booted up ago, with a
	// battery reading ("" for none) and memory bytes installed; up 0 never booted.
	return &Meter{sample: sample, up: up, battery: battery, memory: memory}
}

func (m *Meter) Sample(ctx context.Context) (activity.Sample, error) {
	if err := ctx.Err(); err != nil {
		return activity.Sample{}, err
	}
	s := m.sample
	s.CPU, s.Memory = top(s.CPU), top(s.Memory)
	return s, nil
}

func top(ps []activity.Process) []activity.Process {
	out := slices.Clone(ps)
	if out == nil {
		out = []activity.Process{}
	}
	slices.SortStableFunc(out, func(a, b activity.Process) int {
		switch {
		case a.Value > b.Value:
			return -1
		case a.Value < b.Value:
			return 1
		}
		return 0
	})
	if len(out) > activity.Top {
		out = out[:activity.Top]
	}
	return out
}

func (m *Meter) Uptime(context.Context) (time.Duration, bool) { return m.up, m.up > 0 }

func (m *Meter) Battery(context.Context) string { return m.battery }

func (m *Meter) Memory(context.Context) int64 { return m.memory }
