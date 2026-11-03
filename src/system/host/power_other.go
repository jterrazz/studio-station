//go:build !darwin || !cgo

package host

import (
	"context"

	"github.com/jterrazz/studio-station/src/system"
)

type noPower struct{}

func newPower() system.Power { return noPower{} }

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
