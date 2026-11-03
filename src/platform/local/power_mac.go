package local

import (
	"context"
	"time"

	"github.com/jterrazz/studio-station/src/platform/darwin"
	"github.com/jterrazz/studio-station/src/system"
)

const watchEvery = time.Second

type macPower struct {
	iokit darwin.Power
}

func newPower() system.Power {
	if !darwin.Supported {
		return noPower{}
	}
	return &macPower{}
}

func (p *macPower) HoldIdle(name string) (func(), error) { return darwin.HoldIdle(name) }

func (p *macPower) SetClamshellSleep(disabled bool) error {
	return darwin.SetClamshellSleep(disabled)
}

func (p *macPower) Read() (system.PowerReading, error) {
	r := p.iokit.Read()
	return system.PowerReading{
		Lid: r.Lid, Source: r.Source, HasBattery: r.HasBattery, Percent: r.Percent,
		Charging: r.Charging, LowPower: r.LowPower, Thermal: r.Thermal,
	}, nil
}

// invariant: the watch compares a reading with the last one a second and on
// every IOPowerSources notification, and sends one Event per field that moved,
// so a falling percent ends a session within the second it is posted; Low
// Power Mode is read by the guard loop's own poll.
func (p *macPower) Watch(ctx context.Context) <-chan system.PowerEvent {
	return watchReadings(ctx, p.Read, watchEvery, darwin.PowerSourcesChanged(ctx))
}

type noPower struct{}

func (noPower) HoldIdle(string) (func(), error) { return nil, system.ErrPowerUnsupported }

func (noPower) SetClamshellSleep(bool) error { return system.ErrPowerUnsupported }

func (noPower) Read() (system.PowerReading, error) {
	return system.PowerReading{}, system.ErrPowerUnsupported
}

func (noPower) Watch(context.Context) <-chan system.PowerEvent {
	out := make(chan system.PowerEvent)
	close(out)
	return out
}
