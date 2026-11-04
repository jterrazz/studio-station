package wire

import (
	"context"
	"testing"

	awakefake "github.com/jterrazz/studio-station/src/awake/adapter/fake"
	"github.com/jterrazz/studio-station/src/awake/contract/contracttest"
	"github.com/jterrazz/studio-station/src/catalog"
	coremachine "github.com/jterrazz/studio-station/src/core/machine"
)

func TestALaptopAndADesktopDifferByWhatTheirPowerReports(t *testing.T) {
	laptop := profileOf(context.Background(), nil, "host", "macbook", awakefake.NewPower(contracttest.Laptop()))
	desktop := profileOf(context.Background(), nil, "host", "mini", awakefake.NewPower(contracttest.Desktop()))
	if !laptop.Caps.Battery || !laptop.Caps.Lid || !laptop.Caps.Engine || !laptop.Caps.VMs || !laptop.Caps.Jobs {
		t.Errorf("the laptop's capabilities: %+v", laptop.Caps)
	}
	if desktop.Caps.Battery || desktop.Caps.Lid || !desktop.Caps.Thermal || !desktop.Caps.Engine {
		t.Errorf("the desktop's capabilities: %+v", desktop.Caps)
	}
	if laptop.Kind != coremachine.Mac || legacyKind(laptop) != catalog.Host || laptop.Ref.Name != "macbook" {
		t.Errorf("the laptop's profile: %+v", laptop)
	}
}

func TestAPinnedVMRunsNoEngineAndHoldsNoPower(t *testing.T) {
	for pin, want := range map[string]catalog.MachineKind{"linux": catalog.Linux, "macos": catalog.MacOS} {
		p := profileOf(context.Background(), nil, pin, "dev", awakefake.NewPower(contracttest.Laptop()))
		if p.Kind != coremachine.Workspace || legacyKind(p) != want || p.Caps.Engine || p.Caps.Battery {
			t.Errorf("%s: %+v", pin, p)
		}
	}
}
