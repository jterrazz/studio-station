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

func PageHint(st token.Styles, h page.Hint) string {
	line := st.Muted.Render(token.Arrow+" ") + st.Text.Render(h.Command)
	if h.Text != "" {
		line += " " + st.Muted.Render(h.Text)
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
	line := st.Muted.Render("no " + e.Noun)
	if e.Hint != nil {
		line += atom.Spaces(token.Gap) + Hint(st, *e.Hint, m, keys)
	}
	return line
}

func ErrorLines(st token.Styles, e page.Error) []string {
	return []string{st.Error.Render("Error:") + " " + st.Text.Render(e.Message), st.Muted.Render("Hint:") + " " + st.Text.Render(e.Hint)}
}

func StatusError(st token.Styles, e page.Error, width int) string {
	head := atom.Tone(st, page.Broken).Render(token.MarkBroken) + " "
	hint := atom.Spaces(token.Gap) + st.Muted.Render(token.Arrow+" "+e.Hint)
	room := max(width-atom.Width(head)-atom.Width(hint)-1, 1)
	return head + st.Text.Render(atom.End(e.Message, room)) + hint
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

func Confirm(st token.Styles, c page.Confirm, yes bool) []string {
	lines := []string{st.Title.Render(c.Question)}
	for _, l := range c.Lines {
		text := atom.Show(l)
		if l.Kind == page.KindCommand {
			text = token.Arrow + " " + text
		}
		lines = append(lines, st.Text.Render(text))
	}
	verb := c.Yes
	if verb == "" {
		verb = "Yes"
	}
	yesStyle, noStyle := st.Button, st.Idle
	if !yes {
		yesStyle, noStyle = st.Idle, st.Button
	}
	return append(lines, "", yesStyle.Render(verb)+atom.Spaces(token.Gap)+noStyle.Render("No"))
}
