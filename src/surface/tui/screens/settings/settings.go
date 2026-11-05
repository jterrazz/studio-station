package settingsscreen

import (
	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/page"
	pagesettings "github.com/jterrazz/studio-station/src/pages/settings"

	"github.com/jterrazz/studio-station/src/surface/tui/screens/gesture"
)

func Gesture(a page.Action, rows []page.Row) (gesture.Gesture, bool) {
	// invariant: a checked setting is read from macOS, never set: a, d and o
	// leave it alone, and f opens the System Settings pane that fixes it.
	var set []page.Row
	for _, r := range rows {
		if _, checked := pane(r.ID); !checked {
			set = append(set, r)
		}
	}
	switch a.Key {
	case pagesettings.KeyApply:
		return gesture.Each(set, gesture.V("Apply", "Applying", "Applied"), func(r page.Row) []string { return []string{"settings", "apply", r.ID} }), len(set) > 0
	case pagesettings.KeyOff:
		return gesture.Each(set, gesture.V("Turn off", "Turning off", "Turned off"), func(r page.Row) []string { return []string{"settings", "off", r.ID} }), len(set) > 0
	case pagesettings.KeyDiff:
		if len(set) == 1 {
			return gesture.Gesture{Read: []string{"settings", "diff", set[0].ID}}, true
		}
	case pagesettings.KeyFix:
		if len(rows) == 1 {
			if url, ok := pane(rows[0].ID); ok && url != "" {
				return gesture.Gesture{Open: url}, true
			}
		}
	}
	return gesture.Gesture{}, false
}

func pane(name string) (string, bool) {
	for _, c := range catalog.CheckedSettings {
		if c.Name == name {
			return c.Pane, true
		}
	}
	return "", false
}
