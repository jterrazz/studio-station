package calls

import (
	"context"
	"time"

	"github.com/jterrazz/studio-station/src/activity"
	"github.com/jterrazz/studio-station/src/activity/contract"
	awakecontract "github.com/jterrazz/studio-station/src/awake/contract"
	"github.com/jterrazz/studio-station/src/core/call"
	"github.com/jterrazz/studio-station/src/core/machine"
	"github.com/jterrazz/studio-station/src/core/section"
	diskcontract "github.com/jterrazz/studio-station/src/disk/contract"
)

type Deps struct {
	Meter contract.Meter
	// value: this machine by name and kind.
	Name string
	Kind machine.Kind
	// value: the activity the engine of this Mac keeps of a machine, by
	// name, and when that engine started: ok false when it keeps none of
	// it, an error when it does not answer; nil on a machine whose engine
	// is not its own (a VM's).
	Kept  func(ctx context.Context, machine string) (kept activity.Kept, since time.Time, ok bool, err error)
	Sleep func(ctx context.Context, d time.Duration) error
	// value: the clock a kept history's freshness is judged by; set with
	// Kept.
	Now func() time.Time
	// value: the process that holds a VM's guest on this Mac, by the VM's
	// name, from its engine: 0 when none does; nil on a VM.
	Holder func(ctx context.Context, vm string) (pid int, err error)
	// value: the owners of the facts a sample composes in: free space is
	// the Disk family's, thermal and battery Awake's Power; nil leaves the
	// fact out (a VM's thermal and battery are its Mac's).
	Volume diskcontract.DiskUsage
	Power  awakecontract.Power
}

func All(d Deps) []call.Handler {
	return []call.Handler{call.Bind(Sample, d), call.BindWatch(Read, d)}
}

type SampleIn struct{}

var Sample = call.Call[Deps, SampleIn, activity.Reading]{
	Name: "activity.sample", Section: section.Activity, Where: call.AtTarget, Applies: activity.Applies,
	Run: func(ctx context.Context, d Deps, _ SampleIn, _ call.Progress) (activity.Reading, error) {
		return compose(ctx, d)
	},
}

func compose(ctx context.Context, d Deps) (activity.Reading, error) {
	r, err := Measure(ctx, d.Meter)
	if err != nil {
		return r, err
	}
	if d.Volume != nil {
		if total, free, err := d.Volume.Volume(ctx); err == nil {
			r.Sample.DiskTotal, r.Sample.DiskFree = total, free
		}
	}
	if d.Power != nil {
		if p, err := d.Power.Read(); err == nil {
			if d.Power.Capabilities().Thermal {
				r.Sample.Thermal = p.Thermal
			}
			if p.HasBattery {
				r.Sample.Battery = &activity.Battery{Percent: p.Percent, Charging: p.Charging, OnBattery: p.OnBattery(), Health: p.Health, Cycles: p.Cycles}
			}
		}
	}
	return r, nil
}

func Measure(ctx context.Context, m contract.Meter) (activity.Reading, error) {
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
