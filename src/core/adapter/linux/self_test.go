package linux

import (
	"testing"

	"github.com/jterrazz/studio-station/src/core/contract"
	"github.com/jterrazz/studio-station/src/core/contract/contracttest"
	"github.com/jterrazz/studio-station/src/core/machine"
)

func TestTheLinuxSelfKeepsTheContract(t *testing.T) {
	contracttest.Self(t, func(_ *testing.T, s contracttest.Scene) contract.Self {
		self := New(s.Name)
		self.hardware = func() (machine.System, machine.Hardware) {
			return machine.System{Version: "6.12.0"}, machine.Hardware{Memory: 8 << 30}
		}
		return self
	}, contracttest.LinuxWorkspace())
}
