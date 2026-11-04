package awake

import (
	"strconv"
	"time"

	"github.com/jterrazz/studio-station/src/core/machine"
)

const (
	Thermal  = "thermal"
	Battery  = "battery"
	LowPower = "low_power"
	Lid      = "lid"
)

const (
	BatteryEnds  = 20
	BatteryWarns = 30
	ThermalHeavy = 2
	ThermalEnds  = 3
	HeavyLimit   = 5 * time.Minute
)

type Held struct {
	Lid     bool
	Battery bool
}

type Trip struct {
	Guard  string
	Detail string
	Words  string
}

type Guard struct {
	Name string
	// invariant: a guard exists only on a Mac whose reported capabilities
	// carry the fact it reads, never by its model.
	Needs func(machine.Capabilities) bool
	Trip  func(r Reading, h Held, heavyFor time.Duration) (Trip, bool)
}

func Guards() []Guard {
	// invariant: the order is the precedence: the first guard past its
	// threshold names the end.
	return []Guard{
		{Name: Thermal, Needs: func(c machine.Capabilities) bool { return c.Thermal }, Trip: thermal},
		{Name: Battery, Needs: func(c machine.Capabilities) bool { return c.Battery }, Trip: battery},
		{Name: LowPower, Needs: func(machine.Capabilities) bool { return true }, Trip: lowPower},
		{Name: Lid, Needs: func(c machine.Capabilities) bool { return c.Lid && c.Battery }, Trip: lid},
	}
}

func Active(c machine.Capabilities) []Guard {
	var out []Guard
	for _, g := range Guards() {
		if g.Needs(c) {
			out = append(out, g)
		}
	}
	return out
}

func Check(c machine.Capabilities, r Reading, h Held, heavyFor time.Duration) (Trip, bool) {
	for _, g := range Active(c) {
		if t, ok := g.Trip(r, h, heavyFor); ok {
			return t, true
		}
	}
	return Trip{}, false
}

func thermal(r Reading, _ Held, heavyFor time.Duration) (Trip, bool) {
	switch {
	case r.Thermal >= ThermalEnds:
		return Trip{Thermal, "thermal " + Level(r.Thermal), "the Mac is under serious thermal pressure"}, true
	case r.Thermal == ThermalHeavy && heavyFor >= HeavyLimit:
		return Trip{Thermal, "thermal heavy for 5m lid closed on battery", "the Mac is under heavy thermal pressure"}, true
	}
	return Trip{}, false
}

func battery(r Reading, _ Held, _ time.Duration) (Trip, bool) {
	if !r.OnBattery() || r.Percent > BatteryEnds {
		return Trip{}, false
	}
	p := strconv.Itoa(r.Percent) + "%"
	return Trip{Battery, "battery " + p + " on battery", "the battery is at " + p + " on battery"}, true
}

func lowPower(r Reading, _ Held, _ time.Duration) (Trip, bool) {
	if !r.LowPower {
		return Trip{}, false
	}
	return Trip{LowPower, "Low Power Mode on", "Low Power Mode is on"}, true
}

func lid(r Reading, h Held, _ time.Duration) (Trip, bool) {
	if !h.Lid || h.Battery || !r.LidClosedOnBattery() {
		return Trip{}, false
	}
	return Trip{Lid, "lid closed on battery", "the lid is closed on battery"}, true
}
