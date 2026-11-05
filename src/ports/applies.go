package ports

import (
	"github.com/jterrazz/studio-station/src/core/machine"
	"github.com/jterrazz/studio-station/src/core/section"
)

var Applies = section.Shown(section.Ports)

func DeclaredOnAMac(p machine.Profile) machine.Verdict {
	// invariant: a declaration is kept by a Mac, for itself and its VMs: a
	// VM's port is declared from its Mac.
	if p.Kind == machine.Mac {
		return machine.Yes
	}
	return machine.Verdict{Why: p.Ref.Name + "'s ports are declared from its Mac", Fix: "run it on its Mac, naming " + p.Ref.Name}
}
