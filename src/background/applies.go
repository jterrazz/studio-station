package background

import (
	"slices"

	"github.com/jterrazz/studio-station/src/core/machine"
	"github.com/jterrazz/studio-station/src/core/section"
)

var Applies = machine.All(shown, managed)

func shown(p machine.Profile) machine.Verdict {
	if !slices.Contains(section.For(p.Kind), section.Background) {
		return machine.No("a " + string(p.Kind) + " has no Background section")
	}
	return machine.Yes
}

func managed(p machine.Profile) machine.Verdict {
	if !p.Caps.Jobs {
		return machine.No(p.Ref.Name + " has no job manager station reads")
	}
	return machine.Yes
}
