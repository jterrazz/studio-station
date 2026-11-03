package organism

import (
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/jterrazz/studio-station/src/page"
	"github.com/jterrazz/studio-station/src/ui/atom"
	"github.com/jterrazz/studio-station/src/ui/molecule"
	"github.com/jterrazz/studio-station/src/ui/token"
)

type Binding struct {
	Key     string
	Label   string
	Group   string
	Mutates bool
}

const (
	GroupTab    = "This tab"
	GroupList   = "List"
	GroupGlobal = "Global"
)

func Context(c page.Context) string {
	kind := c.Kind
	if kind == "" {
		kind = "host"
	}
	if !c.Registered || c.Machine == "" {
		return kind + token.Sep + "unregistered"
	}
	return c.Machine + token.Sep + kind
}

func Header(st token.Styles, command, context string, width int) []string {
	left := atom.Spaces(token.Margin) + st.Title.Render("station")
	plain := atom.Spaces(token.Margin) + "station"
	if path := strings.TrimSpace(strings.TrimPrefix(command, "station")); path != "" {
		left += " " + st.Text.Render(path)
		plain += " " + path
	}
	line := left
	if context != "" {
		line += atom.Spaces(max(width-1-atom.Width(plain)-atom.Width(context), 1)) + st.Muted.Render(context)
	}
	return []string{line, Rule(st, width)}
}

func Rule(st token.Styles, width int) string {
	return atom.Spaces(token.Margin) + st.Subtle.Render(atom.Repeat(token.Rule, max(width-2, 1)))
}

func Footer(st token.Styles, bindings []Binding, busy bool, width int) string {
	last := atom.Key(st, "?", "keys")
	room := width - 1 - token.Margin - atom.Width("? keys")
	var parts []string
	used := 0
	for _, b := range bindings {
		w := atom.Width(atom.KeyName(b.Key)+" "+b.Label) + token.Gap
		if used+w > room {
			break
		}
		used += w
		if busy && b.Mutates {
			parts = append(parts, st.Muted.Render(atom.KeyName(b.Key)+" "+b.Label))
			continue
		}
		parts = append(parts, atom.Key(st, b.Key, b.Label))
	}
	parts = append(parts, last)
	return atom.Spaces(token.Margin) + strings.Join(parts, atom.Spaces(token.Gap))
}

func Sheet(st token.Styles, tab string, bindings []Binding, width, height int) []string {
	groups := []string{GroupTab, GroupList, GroupGlobal}
	cols := make([][]string, len(groups))
	colWidth := max((width-6)/len(groups), 12)
	for gi, g := range groups {
		keyWidth := 0
		for _, b := range bindings {
			if b.Group == g {
				keyWidth = max(keyWidth, atom.Width(atom.KeyName(b.Key)))
			}
		}
		cols[gi] = append(cols[gi], st.Title.Render(atom.Pad(g, colWidth)))
		for _, b := range bindings {
			if b.Group != g {
				continue
			}
			cell := st.Accent.Render(atom.Pad(atom.KeyName(b.Key), keyWidth)) + atom.Spaces(token.Gap) + st.Text.Render(atom.End(b.Label, max(colWidth-keyWidth-token.Gap, 1)))
			cols[gi] = append(cols[gi], cell+atom.Spaces(colWidth-atom.Width(cell)))
		}
	}
	rows := 0
	for _, c := range cols {
		rows = max(rows, len(c))
	}
	body := make([]string, 0, rows+1)
	for i := range rows {
		var line strings.Builder
		for _, c := range cols {
			if i < len(c) {
				line.WriteString(c[i])
			} else {
				line.WriteString(atom.Spaces(colWidth))
			}
		}
		body = append(body, strings.TrimRight(line.String(), " "))
	}
	body = append(body, "", st.Muted.Render("any key closes"))
	frame := Frame(st, "Keys"+token.Sep+tab, body, width-2)
	lines := strings.Split(frame, "\n")
	for len(lines) < height {
		lines = append(lines, "")
	}
	return lines[:min(len(lines), max(height, len(lines)))]
}

func Frame(st token.Styles, title string, body []string, width int) string {
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(st.Colors.Subtle).
		Padding(0, 1).
		Width(max(width, 10))
	head := ""
	if title != "" {
		head = st.Title.Render(title) + "\n"
	}
	return lipgloss.NewStyle().MarginLeft(token.Margin).Render(box.Render(head + strings.Join(body, "\n")))
}

func Modal(st token.Styles, c page.Confirm, yes bool, width int) string {
	return Frame(st, "", molecule.Confirm(st, c, yes), min(width-4, token.ModalWidth))
}

func StatusBar(st token.Styles, m page.Mark, text, hint string, width int) string {
	if text == "" {
		return ""
	}
	if m == page.Broken {
		return atom.Spaces(token.Margin) + molecule.StatusError(st, page.Error{Message: text, Hint: hint}, width-token.Margin)
	}
	line := atom.Spaces(token.Margin) + atom.Mark(st, m) + st.Text.Render(atom.End(text, max(width-6, 1)))
	if hint != "" {
		line += atom.Spaces(token.Gap) + st.Muted.Render(hint)
	}
	return line
}

func Strip(st token.Styles, s page.Strip, width int) string {
	parts := []string{}
	add := func(label string, v page.Value, extra string) {
		if v.IsNone() {
			return
		}
		part := st.Muted.Render(label) + " " + st.Text.Render(atom.Show(v))
		if extra != "" {
			part += " " + st.Muted.Render(extra)
		}
		parts = append(parts, part)
	}
	add("CPU", s.CPU, atom.Spark(s.Spark))
	add("memory", s.Memory, "")
	add("battery", s.Battery, "")
	add("GPU", s.GPU, "")
	if len(s.Net) == 2 {
		parts = append(parts, st.Muted.Render("net")+" "+st.Text.Render(token.KeyDown+" "+atom.Show(s.Net[0])+" "+token.KeyUp+" "+atom.Show(s.Net[1])))
	}
	line := atom.Spaces(token.Margin)
	for i, p := range parts {
		next := p
		if i > 0 {
			next = atom.Spaces(token.Gap) + p
		}
		if atom.Width(line+next) > width-1 {
			break
		}
		line += next
	}
	return line
}

func Detail(st token.Styles, s page.Section, r page.Row, bindings []Binding, width int) []string {
	head := atom.Spaces(token.Margin) + st.Strong.Render(r.Label)
	state := atom.Mark(st, r.Mark)
	if r.Reason != "" {
		state += st.Text.Render(string(r.Reason))
	}
	lines := []string{head + atom.Spaces(max(width-1-atom.Width(head)-atom.Width(state), token.Gap)) + state}
	type fact struct{ key, value string }
	var facts []fact
	for i, c := range s.Columns {
		if i == 0 || i-1 >= len(r.Cells) {
			continue
		}
		key := c.Label
		if key == "" {
			key = c.Key
		}
		if v := Full(r.Cells[i-1]); v != "" {
			facts = append(facts, fact{key, v})
		}
	}
	facts = append(facts, fact{"category", s.Title})
	if v := Full(r.Detail); v != "" {
		facts = append(facts, fact{"detail", v})
	}
	if r.Hint != nil {
		facts = append(facts, fact{"fix", r.Hint.Command})
	}
	var keys []string
	for _, b := range bindings {
		if b.Group == GroupTab {
			keys = append(keys, atom.Key(st, b.Key, b.Label))
		}
	}
	width2 := 0
	for _, f := range facts {
		width2 = max(width2, atom.Width(f.key))
	}
	width2 = max(width2, atom.Width("keys"))
	indent := atom.Spaces(token.Margin + token.Indent)
	room := max(width-1-len(indent)-width2-token.Gap, 8)
	for _, f := range facts {
		lines = append(lines, indent+molecule.Fact(st, f.key, width2, atom.End(f.value, room)))
	}
	if len(keys) > 0 {
		lines = append(lines, indent+st.Strong.Render(atom.Pad("keys", width2))+atom.Spaces(token.Gap)+strings.Join(keys, atom.Spaces(token.Gap)))
	}
	return lines
}

func Full(v page.Value) string {
	if v.Kind == page.KindVersion {
		return v.Text
	}
	return atom.Show(v)
}
