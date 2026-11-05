package calls

import (
	"context"

	"github.com/jterrazz/studio-station/src/activity"
	"github.com/jterrazz/studio-station/src/activity/contract"
	awakecontract "github.com/jterrazz/studio-station/src/awake/contract"
	"github.com/jterrazz/studio-station/src/core/call"
	"github.com/jterrazz/studio-station/src/core/section"
	diskcontract "github.com/jterrazz/studio-station/src/disk/contract"
)

type Deps struct {
	Meter contract.Meter
	// value: the owners of the facts a sample composes in: free space is
	// the Disk family's, thermal and battery Awake's Power; nil leaves the
	// fact out (a VM's thermal and battery are its Mac's).
	Volume diskcontract.DiskUsage
	Power  awakecontract.Power
}

func All(d Deps) []call.Handler {
	return []call.Handler{call.Bind(Sample, d)}
}

type SampleIn struct{}

var Sample = call.Call[Deps, SampleIn, activity.Reading]{
	Name: "activity.sample", Section: section.Activity, Where: call.AtTarget, Applies: activity.Applies,
	Run: func(ctx context.Context, d Deps, _ SampleIn, _ call.Progress) (activity.Reading, error) {
		r, err := Read(ctx, d.Meter)
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
					r.Sample.Battery = &activity.Battery{Percent: p.Percent, Charging: p.Charging, OnBattery: p.OnBattery()}
				}
			}
		}
		return r, nil
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
