package awake

import (
	"github.com/jterrazz/studio-station/src/core/machine"
	"github.com/jterrazz/studio-station/src/core/section"
)

var Applies = machine.All(section.Shown(section.Awake), engined)

func engined(p machine.Profile) machine.Verdict {
	if !p.Caps.Engine {
		return machine.No(p.Ref.Name + " runs no engine to hold it awake")
	}
	return machine.Yes
}
