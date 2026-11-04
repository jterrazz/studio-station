package fake

import "github.com/jterrazz/studio-station/src/world"

const here = "here"

func alone() *world.World {
	w := &world.World{Here: here}
	w.Mac(here)
	return w
}
