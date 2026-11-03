package organism

import (
	"slices"
	"strings"

	"github.com/jterrazz/studio-station/src/page"
	"github.com/jterrazz/studio-station/src/ui/atom"
	"github.com/jterrazz/studio-station/src/ui/molecule"
	"github.com/jterrazz/studio-station/src/ui/token"
)

// value: an External command chained by `&&` breaks after each joint when it
// is wider than the terminal, so it stays whole and copyable (H3, G5).
const joint = " && "

type LineKind int

const (
	LineTitle LineKind = iota
	LineHeader
	LineRow
	LineHint
	LineOther
)

type Line struct {
	Key  string
	Kind LineKind
	Head bool
	Text string
}

type Grid struct {
	Styles  token.Styles
	Section page.Section
	Width   int
	Medium  molecule.Medium
	Keys    molecule.Keys
	Folded  func(key string) bool
	Match   func(r page.Row) bool
}

type column struct {
	kind  page.Kind
	label string
	keep  int
	width int
	cell  func(r page.Row) page.Value
}

type entry struct {
	empty  *page.Empty
	link   *page.Link
	row    page.Row
	prefix string
	head   bool
	key    string
}

func RowKey(section, row string) string { return section + "/" + row }

func shown(r page.Row) string {
	if r.Short != "" {
		return r.Short
	}
	return r.Label
}

func (g Grid) Lines() []Line {
	// invariant: a Summary wider than the narrow width reads as two columns, its
	// Break or its first half on the left (§13.1); each half keeps its own grid.
	if g.Section.Role == page.Summary && g.Width > token.NarrowWidth && len(g.Section.Items) > 1 {
		return g.split()
	}
	return g.lines()
}

func (g Grid) split() []Line {
	half := (len(g.Section.Items) + 1) / 2
	if b := g.Section.Break; b > 0 && b < len(g.Section.Items) {
		half = b
	}
	left, right := g, g
	left.Section.Items, right.Section.Items = g.Section.Items[:half], g.Section.Items[half:]
	natural := func(lines []Line) int {
		n := 0
		for _, l := range lines {
			n = max(n, atom.Width(l.Text))
		}
		return n
	}
	lw, rw := natural(left.lines()), natural(right.lines())
	leftWidth := max(lw+1, token.MinWidth)
	if lw+rw >= g.Width {
		leftWidth = max(g.Width/2, g.Width-rw)
	}
	if g.Width+1-leftWidth < token.MinWidth {
		return g.lines()
	}
	left.Width, right.Width = leftWidth, g.Width+1-leftWidth
	l, r := left.lines(), right.lines()
	out := make([]Line, 0, len(l))
	for i := range l {
		line := l[i]
		if i < len(r) {
			line.Text += atom.Spaces(leftWidth-1-atom.Width(line.Text)) + r[i].Text
		}
		out = append(out, line)
	}
	return append(out, r[min(len(l), len(r)):]...)
}

func (g Grid) lines() []Line {
	if g.Keys == nil {
		g.Keys = molecule.NoKeys
	}
	entries := g.entries()
	cols, labelWidth := g.layout(entries)
	var out []Line
	if header := g.header(cols, labelWidth); header != "" && len(entries) > 0 {
		out = append(out, Line{Kind: LineHeader, Text: header})
	}
	factWidth := 0
	for _, it := range g.Section.Items {
		if it.Fact != nil {
			factWidth = max(factWidth, atom.Width(it.Fact.Key))
		}
	}
	at := 0
	for _, it := range g.Section.Items {
		at = g.item(&out, it, entries, at, cols, labelWidth, factWidth)
	}
	return out
}

func (g Grid) item(out *[]Line, it page.Item, entries []entry, at int, cols []column, labelWidth, factWidth int) int {
	st, indent := g.Styles, atom.Spaces(token.Margin+token.Indent)
	switch {
	case it.Row != nil, it.Group != nil:
		n := countEntries(it)
		for _, e := range entries[at : at+n] {
			switch {
			case e.empty != nil && e.key != "":
				*out = append(*out, Line{Kind: LineOther, Text: atom.Spaces(token.Margin+token.Indent) + g.Styles.Subtle.Render(e.prefix) + atom.Spaces(token.MarkWidth) + molecule.Empty(g.Styles, *e.empty, g.Medium, g.Keys)})
			case e.link != nil && e.key != "":
				*out = append(*out, Line{Kind: LineOther, Text: g.link(e.prefix, *e.link, labelWidth)})
			case e.key != "":
				*out = append(*out, g.row(e, cols, labelWidth)...)
			}
		}
		return at + n
	case it.Fact != nil:
		*out = append(*out, Line{Kind: LineOther, Text: indent + molecule.Fact(st, it.Fact.Key, factWidth, atom.Show(it.Fact.Value))})
	case it.Empty != nil:
		*out = append(*out, Line{Kind: LineOther, Text: indent + molecule.Empty(st, *it.Empty, g.Medium, g.Keys)})
	case it.Error != nil:
		for _, l := range molecule.ErrorLines(st, *it.Error) {
			*out = append(*out, Line{Kind: LineOther, Text: indent + l})
		}
	case it.Note != nil:
		for _, l := range molecule.Note(st, *it.Note, max(g.Width-1-len(indent), 10)) {
			*out = append(*out, Line{Kind: LineOther, Text: indent + l})
		}
	case it.Link != nil:
		*out = append(*out, Line{Kind: LineOther, Text: g.link("", *it.Link, labelWidth)})
	}
	return at
}

func (g Grid) link(prefix string, l page.Link, labelWidth int) string {
	st := g.Styles
	label := atom.End(l.Label, max(labelWidth-atom.Width(prefix), 1))
	line := atom.Spaces(token.Margin+token.Indent) + st.Subtle.Render(prefix) + atom.Spaces(token.MarkWidth) + st.Strong.Render(label) + atom.Spaces(labelWidth-atom.Width(prefix)-atom.Width(label))
	if s := atom.Show(l.Value); s != "" {
		line += atom.Spaces(token.Gap) + st.Text.Render(s)
	}
	if s := atom.Show(l.Detail); s != "" {
		line += atom.Spaces(token.Gap) + st.Muted.Render(atom.Fit(l.Detail, max(g.avail()-atom.Width(line)+token.Margin+token.Indent+token.MarkWidth, 8)))
	}
	return line
}

func countEntries(it page.Item) int {
	switch {
	case it.Row != nil:
		return 1
	case it.Group != nil:
		n := 1
		for _, c := range it.Group.Items {
			n += countEntries(c)
		}
		return n
	case it.Link != nil, it.Empty != nil:
		return 1
	}
	return 0
}

func (g Grid) entries() []entry {
	var out []entry
	var walk func(items []page.Item, bars string, depth int, hidden bool)
	walk = func(items []page.Item, bars string, depth int, hidden bool) {
		last := -1
		for i, it := range items {
			if it.Row != nil || it.Group != nil || (depth > 0 && (it.Link != nil || it.Empty != nil)) {
				last = i
			}
		}
		for i, it := range items {
			if it.Row == nil && it.Group == nil && (depth == 0 || (it.Link == nil && it.Empty == nil)) {
				continue
			}
			prefix := ""
			next := bars
			if depth > 0 {
				if i == last {
					prefix, next = bars+token.TreeEnd, bars+atom.Spaces(token.TreeStep)
				} else {
					prefix, next = bars+token.TreeMid, bars+token.TreeBar
				}
			}
			if it.Empty != nil {
				e := entry{empty: it.Empty, prefix: prefix, key: "empty"}
				if hidden {
					e.key = ""
				}
				out = append(out, e)
				continue
			}
			if it.Link != nil {
				e := entry{link: it.Link, prefix: prefix, key: "link"}
				if hidden {
					e.key = ""
				}
				out = append(out, e)
				continue
			}
			if it.Row != nil {
				out = append(out, g.entry(*it.Row, prefix, false, hidden))
				continue
			}
			e := g.entry(it.Group.Head, prefix, true, hidden)
			out = append(out, e)
			shut := hidden || (g.Folded != nil && g.Folded(e.keyOr(g.Section.ID)))
			walk(it.Group.Items, next, depth+1, shut)
		}
	}
	walk(g.Section.Items, "", 0, false)
	return out
}

func (e entry) keyOr(section string) string {
	if e.key != "" {
		return e.key
	}
	return RowKey(section, e.row.ID)
}

func (g Grid) entry(r page.Row, prefix string, head, hidden bool) entry {
	key := RowKey(g.Section.ID, r.ID)
	if hidden || (g.Match != nil && !g.Match(r)) {
		key = ""
	}
	return entry{row: r, prefix: prefix, head: head, key: key}
}

func (g Grid) layout(entries []entry) (cols []column, labelWidth int) {
	for i, c := range g.Section.Columns {
		if i == 0 || c.Show == page.PipeOnly {
			continue
		}
		at := i - 1
		if c.Kind == page.KindLevels {
			for li, level := range c.Levels {
				cols = append(cols, column{kind: page.KindSize, label: level, keep: c.Keep, cell: levelCell(at, level, li == 0)})
			}
			continue
		}
		cols = append(cols, column{kind: c.Kind, label: c.Label, keep: c.Keep, cell: positional(at)})
	}
	if len(g.Section.Columns) > 0 {
		labelWidth = atom.Width(g.Section.Columns[0].Label)
	}
	for _, e := range entries {
		if e.key == "" || e.link != nil || e.empty != nil {
			continue
		}
		labelWidth = max(labelWidth, atom.Width(e.prefix)+atom.Width(shown(e.row)))
		for i := range cols {
			cols[i].width = max(cols[i].width, atom.Width(atom.Show(cols[i].cell(e.row))))
		}
	}
	labelWidth = min(labelWidth, token.LabelMax)
	for i := range cols {
		if cols[i].width > 0 && cols[i].label != "" {
			cols[i].width = max(cols[i].width, atom.Width(cols[i].label))
		}
	}
	cols = slices.DeleteFunc(cols, func(c column) bool { return c.width == 0 })
	avail := g.avail()
	fixed := func() int {
		n := labelWidth
		for _, c := range cols {
			n += token.Gap + c.width
		}
		return n
	}
	order := make([]int, len(cols))
	for i := range order {
		order[i] = i
	}
	slices.SortStableFunc(order, func(a, b int) int { return cols[a].keep - cols[b].keep })
	for _, i := range order {
		if over := fixed() - avail; over > 0 {
			cols[i].width = max(cols[i].width-over, min(atom.Floor(cols[i].kind), cols[i].width))
		}
	}
	if over := fixed() - avail; over > 0 {
		labelWidth = max(labelWidth-over, min(token.LabelMin, labelWidth))
	}
	for _, i := range order {
		if fixed() <= avail {
			break
		}
		cols[i].width = -1
	}
	cols = slices.DeleteFunc(cols, func(c column) bool { return c.width < 0 })
	return cols, labelWidth
}

func (g Grid) avail() int {
	return g.Width - 1 - token.Margin - token.Indent - token.MarkWidth
}

func positional(at int) func(page.Row) page.Value {
	return func(r page.Row) page.Value {
		if at < len(r.Cells) {
			return r.Cells[at]
		}
		return page.None()
	}
}

func levelCell(at int, level string, first bool) func(page.Row) page.Value {
	return func(r page.Row) page.Value {
		if at >= len(r.Cells) {
			return page.None()
		}
		if c := r.Cells[at]; c.Kind != page.KindLevels {
			if first {
				return c
			}
			return page.None()
		}
		for _, l := range r.Cells[at].Items {
			if l.Label == level && l.N != 0 {
				return l
			}
		}
		return page.None()
	}
}

func (g Grid) header(cols []column, labelWidth int) string {
	labeled := false
	for _, c := range cols {
		labeled = labeled || c.label != ""
	}
	if !labeled {
		return ""
	}
	label := ""
	if len(g.Section.Columns) > 0 {
		label = g.Section.Columns[0].Label
	}
	var line strings.Builder
	line.WriteString(atom.Spaces(token.Margin+token.Indent+token.MarkWidth) + atom.Pad(label, labelWidth))
	for _, c := range cols {
		cell := atom.Pad(c.label, c.width)
		if atom.Numeric(c.kind) {
			cell = atom.PadLeft(c.label, c.width)
		}
		line.WriteString(atom.Spaces(token.Gap) + cell)
	}
	return g.Styles.Muted.Render(strings.TrimRight(line.String(), " "))
}

func (g Grid) row(e entry, cols []column, labelWidth int) []Line {
	st, r := g.Styles, e.row
	indent := atom.Spaces(token.Margin + token.Indent)
	lead := indent + st.Subtle.Render(e.prefix) + atom.Mark(st, r.Mark)
	label := atom.End(shown(r), max(labelWidth-atom.Width(e.prefix), 1))
	// invariant: a wrapped hint starts at the label's column (G5) in every
	// section, medium and width: never under a detail whose place depends on
	// the widest cell of the section.
	hintLines := func() []Line {
		pad := indent + atom.Spaces(atom.Width(e.prefix)+token.MarkWidth)
		h := molecule.Hint(st, *r.Hint, g.Medium, g.Keys)
		width := atom.Width(molecule.HintText(*r.Hint, g.Medium, g.Keys))
		if atom.Width(pad)+width < g.Width || !strings.Contains(r.Hint.Command, joint) {
			return []Line{{Kind: LineHint, Key: e.key, Text: pad + h}}
		}
		parts := strings.Split(r.Hint.Command, joint)
		out := make([]Line, 0, len(parts))
		for i, part := range parts {
			if i < len(parts)-1 {
				part += strings.TrimRight(joint, " ")
			}
			text := st.Text.Render(part)
			if i == 0 {
				text = st.Muted.Render(token.Arrow+" ") + text
			} else {
				text = atom.Spaces(token.Gap) + text
			}
			out = append(out, Line{Kind: LineHint, Key: e.key, Text: pad + text})
		}
		return out
	}
	if g.Width < token.MinWidth {
		lines := []Line{{Kind: LineRow, Key: e.key, Head: e.head, Text: lead + st.Strong.Render(label)}}
		var values []string
		for _, c := range cols {
			if s := atom.Show(c.cell(r)); s != "" {
				values = append(values, s)
			}
		}
		if s := atom.Show(r.Detail); s != "" {
			values = append(values, s)
		}
		if len(values) > 0 {
			lines = append(lines, Line{Kind: LineOther, Text: indent + atom.Spaces(token.MarkWidth*2) + st.Muted.Render(strings.Join(values, atom.Spaces(token.Gap)))})
		}
		if r.Hint != nil {
			lines = append(lines, hintLines()...)
		}
		return lines
	}
	var b strings.Builder
	b.WriteString(lead + st.Strong.Render(label) + atom.Spaces(labelWidth-atom.Width(e.prefix)-atom.Width(label)))
	used := labelWidth
	for _, c := range cols {
		v := c.cell(r)
		text := atom.Fit(v, c.width)
		styled := st.Text.Render(text)
		if v.Stale {
			styled = st.Muted.Render(text)
		}
		if text == "" {
			styled = ""
		}
		if atom.Numeric(c.kind) {
			b.WriteString(atom.Spaces(token.Gap+c.width-atom.Width(text)) + styled)
		} else {
			b.WriteString(atom.Spaces(token.Gap) + styled + atom.Spaces(c.width-atom.Width(text)))
		}
		used += token.Gap + c.width
	}
	rest := g.avail() - used
	detail := atom.Show(r.Detail)
	hint, hintWidth := "", 0
	if r.Hint != nil {
		hint = molecule.Hint(st, *r.Hint, g.Medium, g.Keys)
		hintWidth = atom.Width(molecule.HintText(*r.Hint, g.Medium, g.Keys))
	}
	own := false
	switch {
	case detail != "" && hint != "":
		if room := rest - token.Gap*2 - hintWidth; room >= atom.Width(detail) {
			b.WriteString(atom.Spaces(token.Gap) + st.Muted.Render(atom.End(detail, room)) + atom.Spaces(token.Gap) + hint)
		} else {
			if room := rest - token.Gap; room >= 1 {
				b.WriteString(atom.Spaces(token.Gap) + st.Muted.Render(atom.End(detail, room)))
			}
			own = true
		}
	case detail != "":
		if room := rest - token.Gap; room >= 1 {
			b.WriteString(atom.Spaces(token.Gap) + st.Muted.Render(atom.End(detail, room)))
		}
	case hint != "":
		if token.Gap+hintWidth <= rest {
			b.WriteString(atom.Spaces(token.Gap) + hint)
		} else {
			own = true
		}
	}
	lines := []Line{{Kind: LineRow, Key: e.key, Head: e.head, Text: strings.TrimRight(b.String(), " ")}}
	if own {
		lines = append(lines, hintLines()...)
	}
	return lines
}
