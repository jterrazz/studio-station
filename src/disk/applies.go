package disk

import (
	"slices"

	"github.com/jterrazz/studio-station/src/core/machine"
	"github.com/jterrazz/studio-station/src/core/section"
)

func Applies(p machine.Profile) machine.Verdict {
	// invariant: the Disk section is wherever section.For places it; a server
	// keeps nothing of station's to size.
	if slices.Contains(section.For(p.Kind), section.Disk) {
		return machine.Yes
	}
	return machine.No("a " + string(p.Kind) + " keeps nothing of station's on its disk")
}
