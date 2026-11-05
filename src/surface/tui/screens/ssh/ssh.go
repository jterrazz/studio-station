package sshscreen

import (
	"github.com/jterrazz/studio-station/src/page"
	pagemachines "github.com/jterrazz/studio-station/src/pages/machines"
	ui "github.com/jterrazz/studio-station/src/ui/render/tui"

	"github.com/jterrazz/studio-station/src/surface/tui/screens/gesture"
)

func Gesture(p page.Page, a page.Action, rows []page.Row) (gesture.Gesture, bool) {
	if len(rows) != 1 || len(p.Sections) == 0 {
		return gesture.Gesture{}, false
	}
	command := ui.Plain(gesture.Cell(p.Sections[0], rows[0], "command"))
	if command == "" {
		return gesture.Gesture{}, false
	}
	switch a.Key {
	case pagemachines.KeyCopy:
		return gesture.Gesture{Copy: command}, true
	case pagemachines.KeyOpenTerminal:
		return gesture.Gesture{Terminal: command}, true
	}
	return gesture.Gesture{}, false
}
