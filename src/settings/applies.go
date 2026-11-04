package settings

import (
	"slices"

	"github.com/jterrazz/studio-station/src/core/machine"
	"github.com/jterrazz/studio-station/src/core/section"
)

var Applies machine.Applies = func(p machine.Profile) machine.Verdict {
	// invariant: settings are every machine's whose section table holds them.
	if slices.Contains(section.For(p.Kind), section.Settings) {
		return machine.Yes
	}
	return machine.No("settings are not for this machine")
}
