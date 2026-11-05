package tui

import (
	"strings"

	"github.com/jterrazz/studio-station/src/page"
	"github.com/jterrazz/studio-station/src/ui/atom"
	"github.com/jterrazz/studio-station/src/ui/token"
)

const (
	portMachine = 11
	portNumber  = 7
	portWhat    = 18
	portWhere   = 24
	portLead    = 4
)

type PortRow struct {
	// value: a port of a class: its machine, its number, what it is, where
	// it is reached (what it listens on, its route, its link), and what was
	// checked of it (declared, verified) with its mark.
	Key                 string
	Mark                page.Mark
	Machine, Port, What string
	Where               page.Value
	Last                string
	LastMark            page.Mark
}

type PortClass struct {
	// value: a class of ports (SPEC §9.2 Ports): its title, the heads of
	// its columns, its rows, and the fix of its rows said once under them,
	// with the key that does the other way out.
	Mark  page.Mark
	Title string
	Heads []string
	Rows  []PortRow
	Empty string
	Fix   []string
	Or    Pair
}

type PortTable struct {
	Classes []PortClass
	// value: the lines under the classes, the key a line names after it.
	Ending []Check
	Cursor Cursor
}

func (t PortTable) Lines() []string {
	st := token.New(t.Cursor.Dark)
	width := t.Cursor.Width
	var out []Line
	for i, c := range t.Classes {
		if i > 0 {
			out = append(out, Line{})
		}
		title := " " + c.Title
		if g := atom.Glyph(c.Mark); g != "" && c.Mark != page.Unjudged {
			title = " " + atom.Mark(st, c.Mark) + c.Title
		}
		out = append(out, Line{Text: st.Strong.Render(title)})
		if len(c.Heads) > 0 && len(c.Rows) > 0 {
			out = append(out, Line{Text: atom.Spaces(portLead) + st.Muted.Render(columns(c.Heads))})
		}
		for _, r := range c.Rows {
			out = append(out, Line{Key: r.Key, Text: r.line(st, width)})
		}
		if len(c.Rows) == 0 && c.Empty != "" {
			out = append(out, Line{Text: atom.Spaces(portLead) + st.Muted.Render(c.Empty)})
		}
		if len(c.Fix) > 0 {
			fix := "Fix: " + strings.Join(c.Fix, "; ")
			fix = strings.ReplaceAll(fix, " > ", token.Crumb)
			if c.Or.Label != "" {
				fix += ", or " + c.Or.Text + ": " + c.Or.Label
			}
			for _, l := range wrap(fix, max(width-portLead-1, 20)) {
				out = append(out, Line{Text: atom.Spaces(portLead) + l})
			}
		}
	}
	if len(t.Ending) > 0 {
		out = append(out, Line{})
	}
	for _, e := range t.Ending {
		line := " "
		if e.Mark != page.Unjudged {
			line += atom.Mark(st, e.Mark)
		}
		line += e.Label
		if e.Text != "" {
			line += token.Sep + st.Accent.Render(e.Text)
		}
		out = append(out, Line{Text: line})
	}
	return t.Cursor.Lines(out)
}

func columns(heads []string) string {
	widths := []int{portMachine, portNumber, portWhat, portWhere}
	var b strings.Builder
	for i, h := range heads {
		if i < len(widths) {
			b.WriteString(atom.Pad(h, widths[i]))
			continue
		}
		b.WriteString(h)
	}
	return strings.TrimRight(b.String(), " ")
}

func (r PortRow) line(st token.Styles, width int) string {
	line := lead(st, r.Mark) + atom.Pad(atom.End(r.Machine, portMachine-1), portMachine) +
		atom.Pad(r.Port, portNumber) + atom.Pad(atom.End(r.What, portWhat-1), portWhat)
	room := max(width-atom.Width(line)-1, 4)
	if r.Last == "" {
		return line + atom.Fit(r.Where, room)
	}
	where := atom.Pad(atom.Fit(r.Where, portWhere-1), portWhere)
	last := atom.End(r.Last, max(room-portWhere-2, 4))
	if r.LastMark != page.Unjudged {
		last = atom.Tone(st, r.LastMark).Render(atom.Glyph(r.LastMark)) + " " + last
	}
	return line + where + last
}
