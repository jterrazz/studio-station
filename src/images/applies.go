package images

import "github.com/jterrazz/studio-station/src/core/machine"

func Applies(p machine.Profile) machine.Verdict {
	// invariant: images live where VMs are made from them: a Mac, which runs
	// the engine, its hypervisor installed or not yet.
	if !p.Caps.Engine {
		return machine.No(p.Ref.Name + " keeps no images: only a Mac makes VMs")
	}
	return machine.Yes
}
