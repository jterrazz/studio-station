package awake_test

import (
	"slices"
	"testing"

	"github.com/jterrazz/studio-station/src/awake"
	"github.com/jterrazz/studio-station/src/core/machine"
)

func names(gs []awake.Guard) []string {
	out := make([]string, 0, len(gs))
	for _, g := range gs {
		out = append(out, g.Name)
	}
	return out
}

func TestADesktopMacHasNeitherTheBatteryNorTheLidGuard(t *testing.T) {
	got := names(awake.Active(machine.Capabilities{Thermal: true}))
	if want := []string{awake.Thermal, awake.LowPower}; !slices.Equal(got, want) {
		t.Fatalf("a desktop's guards = %v, want %v", got, want)
	}
}

func TestALaptopHasEveryGuardInItsOrder(t *testing.T) {
	got := names(awake.Active(machine.Capabilities{Battery: true, Lid: true, Thermal: true}))
	if want := []string{awake.Thermal, awake.Battery, awake.LowPower, awake.Lid}; !slices.Equal(got, want) {
		t.Fatalf("a laptop's guards = %v, want %v", got, want)
	}
}

func TestABatteryEndsASessionOnlyWhereTheMacReportsOne(t *testing.T) {
	low := awake.Reading{Source: "battery", HasBattery: true, Percent: 15, Lid: "open"}
	if trip, ok := awake.Check(machine.Capabilities{Battery: true, Lid: true, Thermal: true}, low, awake.Held{}, 0); !ok || trip.Guard != awake.Battery {
		t.Fatalf("a laptop at 15%% on battery: %+v, %v; want the battery guard", trip, ok)
	}
	if trip, ok := awake.Check(machine.Capabilities{Thermal: true}, low, awake.Held{}, 0); ok {
		t.Fatalf("a Mac reporting no battery tripped %+v", trip)
	}
}
