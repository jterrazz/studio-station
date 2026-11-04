package fake

import (
	"context"
	"slices"
	"time"

	"github.com/jterrazz/studio-station/src/activity"
	"github.com/jterrazz/studio-station/src/world"
)

type Meter struct {
	// invariant: the counters of one Mac of the world.
	w   *world.World
	mac string
}

func MeterOf(w *world.World, mac string) *Meter { return &Meter{w: w, mac: mac} }

func NewMeter(sample activity.Sample, up time.Duration, battery string, memory int64) *Meter {
	// value: a machine whose counters read sample, booted up ago, with a
	// battery reading ("" for none) and memory bytes installed; up 0 never booted.
	w := &world.World{Here: "here"}
	w.Mac("here").Meter = &world.Meter{Sample: sample, Uptime: up, Battery: battery, Memory: memory}
	return MeterOf(w, "here")
}

func (m *Meter) read() world.Meter {
	m.w.Lock()
	defer m.w.Unlock()
	if r := m.w.Mac(m.mac).Meter; r != nil {
		return *r
	}
	return world.Meter{}
}

func (m *Meter) Sample(ctx context.Context) (activity.Sample, error) {
	if err := ctx.Err(); err != nil {
		return activity.Sample{}, err
	}
	s := m.read().Sample
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

func (m *Meter) Uptime(context.Context) (time.Duration, bool) {
	up := m.read().Uptime
	return up, up > 0
}

func (m *Meter) Battery(context.Context) string { return m.read().Battery }

func (m *Meter) Memory(context.Context) int64 { return m.read().Memory }
