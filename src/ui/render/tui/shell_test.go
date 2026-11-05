package tui_test

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"

	ui "github.com/jterrazz/studio-station/src/ui/render/tui"
)

// invariant: TU-17: under NO_COLOR the cursor row is reverse video, the one
// mark of the cursor a terminal without color still shows.
func TestNoColorCursorIsReverseVideo(t *testing.T) {
	frame := func(noColor bool) string {
		return ui.Shell{Width: 110, Height: 20, NoColor: noColor, ShowSidebar: true, Focus: ui.PaneSidebar, Cursor: 1,
			Sidebar: []ui.Entry{{Label: "MACHINES", Heading: true}, {Label: "This Mac"}, {Label: "mac-mini"}}}.Render()
	}
	line := func(s, label string) string {
		for _, l := range strings.Split(s, "\n") {
			if strings.Contains(l, label) {
				return l
			}
		}
		return ""
	}
	shown := lipgloss.NewStyle().Reverse(true).Render("x")
	reverse, _, _ := strings.Cut(shown, "x")
	if got := line(frame(true), "This Mac"); !strings.Contains(got, reverse) {
		t.Errorf("NO_COLOR cursor row %q has no reverse video", got)
	}
	if got := line(frame(true), "mac-mini"); strings.Contains(got, reverse) {
		t.Errorf("NO_COLOR row off the cursor %q is reverse video", got)
	}
	if got := line(frame(false), "This Mac"); strings.Contains(got, reverse) {
		t.Errorf("a colored cursor row %q is reverse video, not its background", got)
	}
}
