package tui

import (
	"slices"
	"strconv"
	"strings"

	"github.com/jterrazz/studio-station/src/page"
	"github.com/jterrazz/studio-station/src/ui/atom"
	"github.com/jterrazz/studio-station/src/ui/organism"
	"github.com/jterrazz/studio-station/src/ui/token"
)

type Matrix struct {
	// value: the settings matrix as the TUI draws it (vision screen 4): one
	// row per setting, one column per machine, a glyph per cell, the cursor
	// on one cell.
	Page page.Page
	// value: the row the cursor is on, `<section>/<setting>`, and its column.
	Row    string
	Col    int
	Filter string
	// value: the words the title adds (only what needs a look).
	View string
}

const (
	matrixLabel = 30
	matrixCell  = 3
)

func grid(s page.Section) bool {
	return len(s.Columns) > 2 && s.Columns[len(s.Columns)-1].Key == "ok"
}

// value: the machine columns of the matrix, by their headings, in order.
func (m Matrix) heads() []string {
	for _, s := range m.Page.Sections {
		if !grid(s) {
			continue
		}
		out := make([]string, 0, len(s.Columns)-2)
		for _, c := range s.Columns[1 : len(s.Columns)-1] {
			out = append(out, c.Label)
		}
		return out
	}
	return nil
}

func (m Matrix) match(r page.Row) bool {
	needle := strings.ToLower(m.Filter)
	return needle == "" || strings.Contains(strings.ToLower(r.Label), needle) || strings.Contains(strings.ToLower(r.ID), needle)
}

func (m Matrix) Keys() []string {
	// value: the setting rows the cursor moves over, by their line's key.
	var out []string
	for _, s := range m.Page.Sections {
		if !grid(s) {
			continue
		}
		for _, it := range s.Items {
			if it.Row != nil && m.match(*it.Row) {
				out = append(out, organism.RowKey(s.ID, it.Row.ID))
			}
		}
	}
	return out
}

func (m Matrix) Fix() Matrix {
	// value: the cursor kept on a row that is still shown, else on the first.
	keys := m.Keys()
	if !slices.Contains(keys, m.Row) {
		m.Row = ""
		if len(keys) > 0 {
			m.Row = keys[0]
		}
	}
	m.Col = min(max(m.Col, 0), max(len(m.heads())-1, 0))
	return m
}

func (m Matrix) Move(rows, cols int) Matrix {
	m = m.Fix()
	keys := m.Keys()
	if i := slices.Index(keys, m.Row); i >= 0 {
		m.Row = keys[min(max(i+rows, 0), len(keys)-1)]
	}
	m.Col += cols
	return m.Fix()
}

func (m Matrix) Columns() int { return len(m.heads()) }

func (m Matrix) Current() (page.Row, string, bool) {
	// value: the row under the cursor and the machine of its column.
	sid, rid, _ := strings.Cut(m.Row, "/")
	for _, s := range m.Page.Sections {
		if s.ID != sid || !grid(s) {
			continue
		}
		for _, it := range s.Items {
			if it.Row != nil && it.Row.ID == rid {
				machine := ""
				if m.Col+1 < len(s.Columns)-1 {
					machine = s.Columns[m.Col+1].Key
				}
				return *it.Row, machine, true
			}
		}
	}
	return page.Row{}, "", false
}

func (m Matrix) Title(name, machine string) string {
	// value: the title as a list writes it, the filter and its count after the
	// section's name (`Settings (/git) 2 · All machines`), then the view.
	t := name
	if m.Filter != "" {
		t += " (/" + m.Filter + ") " + strconv.Itoa(len(m.Keys()))
	}
	t += token.Sep + machine
	if m.View != "" {
		t += token.Sep + m.View
	}
	return t
}

func (m Matrix) Lines(s Shell, width, room int, focused bool) []string {
	st := token.New(s.Dark)
	heads := m.heads()
	label := 0
	for _, sec := range m.Page.Sections {
		if !grid(sec) {
			continue
		}
		for _, it := range sec.Items {
			if it.Row != nil {
				label = max(label, atom.Width(it.Row.Label))
			}
		}
	}
	label = min(label, matrixLabel)
	widths := make([]int, len(heads))
	for i, h := range heads {
		widths[i] = max(atom.Width(h), matrixCell) + token.Gap
	}
	lead := atom.Spaces(token.Margin + token.MarkWidth + label + 1)
	var lines []string
	key := map[int]string{}
	if sum := m.summary(); sum != "" {
		lines = append(lines, atom.Spaces(token.Margin)+st.Text.Render(atom.End(sum, width-token.Margin-1)), "")
	}
	var head strings.Builder
	head.WriteString(lead)
	for i, h := range heads {
		head.WriteString(" " + atom.Pad(h, widths[i]-1))
	}
	lines = append(lines, st.Muted.Render(head.String()+" OK"))
	shown := false
	for _, sec := range m.Page.Sections {
		if !grid(sec) {
			continue
		}
		var rows []page.Row
		for _, it := range sec.Items {
			if it.Row != nil && m.match(*it.Row) {
				rows = append(rows, *it.Row)
			}
		}
		if len(rows) == 0 {
			continue
		}
		shown = true
		lines = append(lines, atom.Spaces(token.Margin)+st.Strong.Render(sec.Title))
		for _, r := range rows {
			k := organism.RowKey(sec.ID, r.ID)
			at := k == m.Row
			var b strings.Builder
			b.WriteString(atom.Spaces(token.Margin) + atom.Mark(st, r.Mark) + st.Text.Render(atom.Pad(atom.End(r.Label, label), label+1)))
			for i := range heads {
				b.WriteString(m.cell(st, r, i, widths[i], at && focused && i == m.Col))
			}
			line := b.String()
			if len(r.Cells) > len(heads) {
				line += " " + st.Muted.Render(atom.Plain(r.Cells[len(heads)]))
			}
			if at && focused {
				line = s.CursorLine(line, width)
			}
			key[len(lines)] = k
			lines = append(lines, line)
		}
	}
	if !shown {
		text := "Nothing needs a look" + token.Sep + "F shows every setting"
		if m.Filter != "" {
			text = "Nothing matches /" + m.Filter + token.Sep + "esc clears it"
		}
		lines = append(lines, " "+st.Muted.Render(text))
	}
	if away := m.away(); len(away) > 0 {
		lines = append(lines, "")
		for _, a := range away {
			lines = append(lines, atom.Spaces(token.Margin)+st.Muted.Render(atom.End(a, width-token.Margin-1)))
		}
	}
	start := 0
	for i, k := range key {
		if k == m.Row && i >= room {
			start = i - room + 1
		}
	}
	return lines[start:min(len(lines), start+room)]
}

func (m Matrix) cell(st token.Styles, r page.Row, i, width int, cursor bool) string {
	if i >= len(r.Cells) {
		return atom.Spaces(width)
	}
	v := r.Cells[i]
	mark := page.Mark(v.Label)
	text := atom.Glyph(mark)
	if v.Text != "" {
		text += " " + v.Text
	}
	glyph := atom.Tone(st, mark).Render(text)
	if cursor {
		return "[" + glyph + "]" + atom.Spaces(width-atom.Width(text)-2)
	}
	return " " + glyph + atom.Spaces(width-atom.Width(text)-1)
}

func (m Matrix) summary() string {
	for _, s := range m.Page.Sections {
		if s.Role != page.Summary {
			continue
		}
		for _, it := range s.Items {
			if it.Row == nil {
				continue
			}
			parts := make([]string, 0, 1+len(it.Row.Cells))
			parts = append(parts, it.Row.Label)
			for _, c := range it.Row.Cells {
				parts = append(parts, atom.Plain(c))
			}
			return strings.Join(parts, token.Sep)
		}
	}
	return ""
}

// value: each machine whose column is not read now, in its row's words
// (api · stopped · last read 2 d ago; infra · not checked).
func (m Matrix) away() []string {
	var out []string
	for _, s := range m.Page.Sections {
		if s.ID != "machines" {
			continue
		}
		for _, it := range s.Items {
			if it.Row == nil || it.Row.Mark == page.OK {
				continue
			}
			name := it.Row.Short
			if name == "" {
				name = it.Row.Label
			}
			parts := []string{name}
			if len(it.Row.Cells) > 1 {
				parts = append(parts, atom.Plain(it.Row.Cells[1]))
			}
			out = append(out, strings.Join(parts, token.Sep))
		}
	}
	return out
}
