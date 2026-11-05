package molecule

import (
	"strings"

	"github.com/jterrazz/studio-station/src/page"
	"github.com/jterrazz/studio-station/src/ui/atom"
	"github.com/jterrazz/studio-station/src/ui/token"
)

type Medium int

const (
	TTY Medium = iota
	TUI
)

type Keys func(key string) (label string, ok bool)

func NoKeys(string) (string, bool) { return "", false }

func HintText(h page.Hint, m Medium, keys Keys) string {
	if m == TUI && h.Key != "" && !h.External {
		if label, ok := keys(h.Key); ok {
			return atom.KeyName(h.Key) + " " + label
		}
	}
	return token.Arrow + " " + h.Command
}

func Hint(st token.Styles, h page.Hint, m Medium, keys Keys) string {
	if m == TUI && h.Key != "" && !h.External {
		if label, ok := keys(h.Key); ok {
			return atom.Key(st, h.Key, label)
		}
	}
	return st.Muted.Render(token.Arrow+" ") + st.Text.Render(h.Command)
}

func PageHint(st token.Styles, h page.Hint, at int) string {
	// value: a next step, its words after the command at a column `at` wide
	// (the widest command of the page's steps), so the steps read as one list.
	line := st.Muted.Render(token.Arrow+" ") + st.Text.Render(h.Command)
	if h.Text != "" {
		line += atom.Spaces(max(at-atom.Width(h.Command), 0)+token.Gap) + st.Muted.Render(h.Text)
	}
	return line
}

func Counts(st token.Styles, tallies []page.Tally) string {
	parts := make([]string, 0, len(tallies))
	for _, t := range tallies {
		if t.N > 0 {
			parts = append(parts, atom.Tone(st, t.Mark).Render(atom.Count(t)))
		}
	}
	return strings.Join(parts, atom.Spaces(token.Gap))
}

func Empty(st token.Styles, e page.Empty, m Medium, keys Keys) string {
	text := "no " + e.Noun
	if e.Text != "" {
		text = e.Text
	}
	line := st.Muted.Render(text)
	if e.Hint == nil {
		return line
	}
	// invariant: TU-20: on the TUI an empty line names the key that fills
	// it (`No VMs yet. Press n to create one.`); a key the screen does not
	// answer is never named, and a hint with no command is the TUI's alone.
	if m == TUI && e.Hint.Key != "" && !e.Hint.External {
		if label, ok := keys(e.Hint.Key); ok {
			what := e.Hint.Text
			if what == "" {
				what = label
			}
			return st.Muted.Render(text+". Press ") + st.Accent.Render(atom.KeyName(e.Hint.Key)) + st.Muted.Render(" to "+what+".")
		}
	}
	if e.Hint.Command == "" {
		return line
	}
	return line + atom.Spaces(token.Gap) + Hint(st, *e.Hint, m, keys)
}

func ErrorLines(st token.Styles, e page.Error) []string {
	lines := []string{st.Error.Render("Error:") + " " + st.Text.Render(e.Message)}
	if e.Hint != "" {
		lines = append(lines, st.Muted.Render("Hint:")+" "+st.Text.Render(e.Hint))
	}
	return lines
}

func Note(st token.Styles, n page.Note, width int) []string {
	var lines []string
	line := ""
	for _, word := range strings.Fields(n.Text) {
		switch {
		case line == "":
			line = word
		case atom.Width(line)+1+atom.Width(word) <= width:
			line += " " + word
		default:
			lines = append(lines, st.Text.Render(line))
			line = word
		}
	}
	if line != "" {
		lines = append(lines, st.Text.Render(line))
	}
	return lines
}

func Fact(st token.Styles, key string, keyWidth int, value string) string {
	return st.Strong.Render(atom.Pad(key, keyWidth)) + atom.Spaces(token.Gap) + st.Text.Render(value)
}
