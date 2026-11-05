package tui

import (
	"strings"

	"github.com/jterrazz/studio-station/src/page"
	"github.com/jterrazz/studio-station/src/ui/atom"
	"github.com/jterrazz/studio-station/src/ui/token"
)

const (
	Cursor = token.Cursor
	Tick   = token.Tick
	Typed  = token.Typed
)

type Task struct {
	// value: one line of the Progress strip: a command running, its toast, or
	// its failure with the keys that answer it.
	Mark page.Mark
	Text string
	Keys []Binding
}

func (s Shell) progressLines(st token.Styles) []string {
	out := make([]string, 0, len(s.Progress))
	for i, t := range s.Progress {
		lead := atom.Spaces(len(" Progress  "))
		if i == 0 {
			lead = " " + st.Strong.Render("Progress") + "  "
		}
		mark := atom.Mark(st, t.Mark)
		if t.Mark == page.OK {
			mark = atom.Tone(st, page.OK).Render(Tick) + " "
		}
		keys := ""
		if len(t.Keys) > 0 {
			parts := make([]string, 0, len(t.Keys))
			for _, k := range t.Keys {
				parts = append(parts, atom.Key(st, k.Key, k.Label))
			}
			keys = atom.Spaces(token.Gap) + strings.Join(parts, atom.Spaces(token.Gap))
		}
		room := s.Width - atom.Width(lead+mark+keys) - 1
		line := lead + mark + st.Text.Render(atom.End(t.Text, room)) + keys
		if s.Focus == PaneProgress && t.Mark == page.Broken {
			line = s.cursor(st, line, s.Width)
		}
		out = append(out, fit(line, s.Width))
	}
	return out
}
