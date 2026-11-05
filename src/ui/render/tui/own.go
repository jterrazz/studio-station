package tui

import (
	"slices"
	"strings"

	"github.com/jterrazz/studio-station/src/page"
	"github.com/jterrazz/studio-station/src/ui/atom"
	"github.com/jterrazz/studio-station/src/ui/token"
)

type Line struct {
	// value: a line of a pane a screen draws itself, keyed by the list's key
	// of the row it draws, so the cursor lands on it.
	Key, Text string
}

type Cursor struct {
	// value: where the list's cursor is and whether it is drawn, and the
	// room the pane has.
	Key           string
	Focused       bool
	Dark, NoColor bool
	Width, Room   int
}

func (c Cursor) Lines(lines []Line) []string {
	// invariant: the cursor's line is drawn selected and kept in view; the
	// pane scrolls no further than it needs to show it.
	at := slices.IndexFunc(lines, func(l Line) bool { return l.Key != "" && l.Key == c.Key })
	start := 0
	if c.Room > 0 && at >= c.Room {
		start = at - c.Room + 1
	}
	out := make([]string, 0, len(lines))
	s := Shell{Dark: c.Dark, NoColor: c.NoColor}
	for i := start; i < len(lines); i++ {
		text := lines[i].Text
		if c.Focused && i == at {
			text = s.CursorLine(text, c.Width)
		}
		out = append(out, text)
	}
	return out
}

func wrap(words string, width int) []string {
	var out []string
	line := ""
	for _, w := range strings.Fields(words) {
		switch {
		case line == "":
			line = w
		case atom.Width(line)+1+atom.Width(w) > width:
			out, line = append(out, line), w
		default:
			line += " " + w
		}
	}
	if line != "" {
		out = append(out, line)
	}
	return out
}

func lead(st token.Styles, m page.Mark) string {
	if m == page.OK || m == page.Unjudged || atom.Glyph(m) == "" {
		return atom.Spaces(4)
	}
	return " " + atom.Tone(st, m).Render(atom.Glyph(m)) + "  "
}
