package activity

import (
	"slices"

	"github.com/jterrazz/studio-station/src/core/machine"
	"github.com/jterrazz/studio-station/src/core/section"
)

func Applies(p machine.Profile) machine.Verdict {
	// invariant: Activity is on every kind section.For lists it on.
	if slices.Contains(section.For(p.Kind), section.Activity) {
		return machine.Yes
	}
	return machine.No("a " + string(p.Kind) + " has no Activity section")
}
