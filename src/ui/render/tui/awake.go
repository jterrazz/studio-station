package tui

import (
	"strings"

	"github.com/jterrazz/studio-station/src/page"
	"github.com/jterrazz/studio-station/src/ui/atom"
	"github.com/jterrazz/studio-station/src/ui/token"
)

const awakeLabel = 11

type Toggle struct {
	Label string
	On    bool
}

type AwakeLine struct {
	// value: a line under a Mac's state: a label and its words, the keys it
	// offers, the toggles it holds.
	Label   string
	Toggles []Toggle
	Keys    []Pair
	Text    string
}

type AwakeMac struct {
	// value: a Mac's block (vision screen 7): how it is named and reached,
	// its state with its mark, then its lines.
	Key   string
	Title []string
	Mark  page.Mark
	State []string
	Lines []AwakeLine
}

type Awake struct {
	Intro, Ending string
	Macs          []AwakeMac
	Cursor        Cursor
}

func (a Awake) Lines() []string {
	st := token.New(a.Cursor.Dark)
	out := []Line{{Text: " " + a.Intro}}
	for _, m := range a.Macs {
		out = append(out, Line{}, Line{Key: m.Key, Text: " " + st.Strong.Render(strings.Join(m.Title, token.Sep))},
			Line{Text: "   " + atom.Mark(st, m.Mark) + strings.Join(m.State, token.Sep)})
		for _, l := range m.Lines {
			out = append(out, Line{Text: "     " + l.line(st)})
		}
	}
	if a.Ending != "" {
		out = append(out, Line{}, Line{Text: " " + st.Muted.Render(a.Ending)})
	}
	return a.Cursor.Lines(out)
}

func (l AwakeLine) line(st token.Styles) string {
	var b strings.Builder
	if l.Label != "" {
		b.WriteString(atom.Pad(l.Label, awakeLabel))
	}
	for i, t := range l.Toggles {
		mark, word := page.Absent, "off"
		if t.On {
			mark, word = page.OK, "on"
		}
		if i == 0 && l.Label == "" {
			b.WriteString(atom.Pad(t.Label, awakeLabel))
		} else {
			b.WriteString("    " + t.Label + " ")
		}
		b.WriteString(atom.Tone(st, mark).Render(atom.Glyph(mark)) + " " + word)
	}
	if len(l.Toggles) > 0 && len(l.Keys) > 0 {
		b.WriteString("     ")
	}
	keys := make([]string, 0, len(l.Keys))
	named := false
	for _, k := range l.Keys {
		key := st.Accent.Render(k.Label)
		if k.Text != "" {
			key, named = key+" "+k.Text, true
		}
		keys = append(keys, key)
	}
	if named {
		b.WriteString(strings.Join(keys, "   "))
	} else {
		b.WriteString(strings.Join(keys, " "))
	}
	if l.Text != "" {
		if len(l.Keys) > 0 {
			b.WriteString(" ")
		}
		b.WriteString(l.Text)
	}
	return b.String()
}
