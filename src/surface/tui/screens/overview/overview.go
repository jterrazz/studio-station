package overviewscreen

import (
	"strings"

	"github.com/jterrazz/studio-station/src/page"
	pagehome "github.com/jterrazz/studio-station/src/pages/home"
	ui "github.com/jterrazz/studio-station/src/ui/render/tui"

	"github.com/jterrazz/studio-station/src/surface/tui/screens/gesture"
)

func Gesture(p page.Page, a page.Action, rows []page.Row) (gesture.Gesture, bool) {
	// invariant: a Needs-you line's fix is station's when it is a station
	// command: enter runs it; any other fix is yours: enter shows it, to copy
	// or to follow, and runs nothing.
	if a.ID == pagehome.ActionStartEngine {
		return gesture.Gesture{Steps: []gesture.Step{{Words: []string{"engine", "start"}, Doing: "Starting the engine", Done: "Started the engine"}}}, true
	}
	if a.Key != pagehome.KeyFix || len(rows) != 1 {
		return gesture.Gesture{}, false
	}
	r := rows[0]
	var fix string
	for _, sec := range p.Sections {
		if sec.ID == "needs-you" {
			fix = ui.Plain(gesture.Cell(sec, r, "fix"))
		}
	}
	if words, ours := strings.CutPrefix(fix, "station "); ours {
		return gesture.Gesture{Steps: []gesture.Step{{Words: strings.Fields(words), Ask: "Fix " + r.Label + "?", Doing: "Fixing: " + r.Label, Done: "Fixed: " + r.Label}}}, true
	}
	if fix == "" {
		return gesture.Gesture{}, false
	}
	how := "Run it yourself:"
	if strings.HasPrefix(fix, "System Settings") {
		how = "Open it yourself:"
	}
	return gesture.Gesture{Note: []string{r.Label, "", how, fix, "", "station runs nothing here"}}, true
}
