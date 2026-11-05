package diskscreen

import (
	"strings"

	"github.com/jterrazz/studio-station/src/page"
	pagedisk "github.com/jterrazz/studio-station/src/pages/disk"

	"github.com/jterrazz/studio-station/src/surface/tui/screens/gesture"
)

// value: a disk row is `<group>/<what>`: a folder for Misplaced, an app
// for every other group.
func what(r page.Row) string {
	_, w, _ := strings.Cut(r.ID, "/")
	return w
}

func Gesture(a page.Action, rows []page.Row) (gesture.Gesture, bool) {
	var g gesture.Gesture
	switch a.Key {
	case pagedisk.KeyMove:
		g = gesture.Each(rows, gesture.V("Move", "Moving", "Moved"), func(r page.Row) []string { return []string{"disk", "move", what(r)} })
	case pagedisk.KeyFree:
		g = gesture.Each(rows, gesture.V("Free", "Freeing", "Freed"), func(r page.Row) []string { return []string{"disk", "free", what(r)} })
	case pagedisk.KeyErase:
		g = gesture.Each(rows, gesture.V("Erase", "Erasing", "Erased"), func(r page.Row) []string { return []string{"disk", "erase", what(r)} })
	default:
		return g, false
	}
	return g, len(g.Steps) > 0
}
