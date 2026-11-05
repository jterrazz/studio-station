package backgroundscreen

import (
	"github.com/jterrazz/studio-station/src/page"
	pagebackground "github.com/jterrazz/studio-station/src/pages/background"
	ui "github.com/jterrazz/studio-station/src/ui/render/tui"

	"github.com/jterrazz/studio-station/src/surface/tui/screens/gesture"
)

func Gesture(p page.Page, a page.Action, rows []page.Row) (gesture.Gesture, bool) {
	// invariant: a user task is station's to remove; a root one is yours: its
	// command is copied, never run (station never runs sudo).
	if len(rows) != 1 {
		return gesture.Gesture{}, false
	}
	r := rows[0]
	sec, _ := find(p, r.ID)
	root := ui.Plain(gesture.Cell(sec, r, "scope")) == "root"
	switch a.Key {
	case pagebackground.KeyRemove:
		if root {
			return gesture.Gesture{}, false
		}
		return gesture.Each(rows, gesture.V("Remove", "Removing", "Removed"), func(r page.Row) []string { return []string{"background", "remove", r.ID} }), true
	case pagebackground.KeyCopy:
		if !root || r.Hint == nil {
			return gesture.Gesture{}, false
		}
		return gesture.Gesture{Copy: r.Hint.Command}, true
	}
	return gesture.Gesture{}, false
}

func find(p page.Page, id string) (page.Section, bool) {
	for _, sec := range p.Sections {
		for _, it := range sec.Items {
			if it.Row != nil && it.Row.ID == id {
				return sec, true
			}
		}
	}
	return page.Section{}, false
}
