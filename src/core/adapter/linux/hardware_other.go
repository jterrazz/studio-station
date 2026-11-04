//go:build !linux

package linux

import "github.com/jterrazz/studio-station/src/core/machine"

func hardware() (machine.System, machine.Hardware) { return machine.System{}, machine.Hardware{} }
