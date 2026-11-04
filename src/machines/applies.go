package machines

import "github.com/jterrazz/studio-station/src/core/machine"

var CanCreate = machine.All(runsAnEngine, hostsVMs)

var AddsMacs = machine.All(reachesMacs)

func reachesMacs(p machine.Profile) machine.Verdict {
	if !p.Caps.Engine {
		return machine.No(p.Ref.Name + " is a " + string(p.Kind) + ": Macs are added from a Mac")
	}
	return machine.Yes
}

func runsAnEngine(p machine.Profile) machine.Verdict {
	if !p.Caps.Engine {
		return machine.No(p.Ref.Name + " runs no engine: VMs are made on a Mac")
	}
	return machine.Yes
}

func hostsVMs(p machine.Profile) machine.Verdict {
	if !p.Caps.VMs {
		return machine.Verdict{Why: p.Caps.Hypervisor + " is not installed on This Mac", Fix: "station apps install " + p.Caps.Hypervisor}
	}
	return machine.Yes
}
