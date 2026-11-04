package fake

import (
	"cmp"
	"context"
	"errors"
	"slices"

	"github.com/jterrazz/studio-station/src/activity"
	"github.com/jterrazz/studio-station/src/world"
)

var ErrNoProcess = errors.New("no such process")

type Meter struct {
	// invariant: the counters of one Mac of the world.
	w   *world.World
	mac string
}

func MeterOf(w *world.World, mac string) *Meter { return &Meter{w: w, mac: mac} }

func NewMeter(r world.Meter) *Meter {
	// value: a machine whose kernel answers r; the zero Meter answers nothing.
	w := &world.World{Here: "here"}
	w.Mac("here").Meter = &r
	return MeterOf(w, "here")
}

func (m *Meter) read() (world.Meter, bool) {
	m.w.Lock()
	defer m.w.Unlock()
	if r := m.w.Mac(m.mac).Meter; r != nil {
		return *r, true
	}
	return world.Meter{}, false
}

func (m *Meter) Sample(ctx context.Context) (activity.Sample, error) {
	if err := ctx.Err(); err != nil {
		return activity.Sample{}, err
	}
	r, _ := m.read()
	s := r.Sample
	s.Thermal, s.Battery, s.DiskFree, s.DiskTotal = activity.ThermalUnknown, nil, 0, 0
	return s, nil
}

func (m *Meter) Top(ctx context.Context) (activity.Busiest, error) {
	if err := ctx.Err(); err != nil {
		return activity.Busiest{}, err
	}
	r, _ := m.read()
	return activity.Busiest{CPU: top(r.Top.CPU), Memory: top(r.Top.Memory)}, nil
}

func (m *Meter) Process(ctx context.Context, pid int) (activity.Usage, error) {
	if err := ctx.Err(); err != nil {
		return activity.Usage{}, err
	}
	r, _ := m.read()
	u, ok := r.Processes[pid]
	if !ok {
		return activity.Usage{}, ErrNoProcess
	}
	return u, nil
}

func top(ps []activity.Process) []activity.Process {
	out := slices.Clone(ps)
	if out == nil {
		out = []activity.Process{}
	}
	slices.SortStableFunc(out, func(a, b activity.Process) int { return cmp.Compare(b.Value, a.Value) })
	return out[:min(len(out), activity.Top)]
}
