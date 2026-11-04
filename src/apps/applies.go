package apps

import (
	"slices"

	"github.com/jterrazz/studio-station/src/core/machine"
	"github.com/jterrazz/studio-station/src/core/section"
)

func Applies(p machine.Profile) machine.Verdict {
	// invariant: the Apps section is wherever section.For places it; a server
	// keeps no apps of station's.
	if slices.Contains(section.For(p.Kind), section.Apps) {
		return machine.Yes
	}
	return machine.No("a " + string(p.Kind) + " keeps no apps of station's")
}
