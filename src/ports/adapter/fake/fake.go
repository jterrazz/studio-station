package fake

import "github.com/jterrazz/studio-station/src/world"

const thisMac = "here"

func alone() *world.World {
	w := &world.World{ThisMac: thisMac}
	w.Mac(thisMac)
	return w
}
