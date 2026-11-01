package output

import (
	"encoding/json"
	"io"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/jterrazz/studio-station/src/present"
	"github.com/jterrazz/studio-station/src/theme"
)

type Renderer interface {
	Render(w io.Writer, a present.Answer) error
}

func For(c Context, t theme.Theme) Renderer {
	switch c.Mode {
	case JSON:
		return jsonRenderer{}
	case Pipe:
		return pipeRenderer{}
	case TTY:
		return ttyRenderer{width: c.Width, theme: t}
	}
	return pipeRenderer{}
}

func TTYString(d present.Document, width int, t theme.Theme) string {
	return ttyRenderer{width: width, theme: t}.document(d)
}

func TTYBody(d present.Document, width int, t theme.Theme) string {
	r := ttyRenderer{width: width, theme: t}
	var b strings.Builder
	widths := columnWidths(d.Blocks)
	for i, block := range d.Blocks {
		if i > 0 {
			b.WriteByte('\n')
		}
		r.block(&b, block, widths)
	}
	return b.String()
}

type jsonRenderer struct{}

func (jsonRenderer) Render(w io.Writer, a present.Answer) error {
	body, err := json.MarshalIndent(a.JSON, "", "  ")
	if err != nil {
		return err
	}
	_, err = w.Write(append(body, '\n'))
	return err
}

type pipeRenderer struct{}

func (pipeRenderer) Render(w io.Writer, a present.Answer) error {
	if a.Piped != "" {
		_, err := io.WriteString(w, a.Piped)
		return err
	}
	var b strings.Builder
	for _, block := range a.Doc.Blocks {
		pipeBlock(&b, block)
	}
	_, err := io.WriteString(w, b.String())
	return err
}

func pipeBlock(b *strings.Builder, block present.Block) {
	switch v := block.(type) {
	case present.Section:
		counts := orderedCounts(v.Counts)
		fields := make([]string, 0, len(counts)+1)
		fields = append(fields, "# "+v.Title)
		for _, c := range counts {
			fields = append(fields, countText(c))
		}
		pipeLine(b, fields)
		for _, inner := range v.Blocks {
			pipeBlock(b, inner)
		}
	case present.Table:
		for _, row := range v.Rows {
			fields := make([]string, 0, len(row.Cells)+1)
			fields = append(fields, row.Mark.Glyph())
			for _, c := range row.Cells {
				fields = append(fields, c.Text)
			}
			pipeLine(b, fields)
		}
	case present.KeyValues:
		for _, p := range v.Pairs {
			pipeLine(b, []string{p.Key, p.Value.Text})
		}
	case present.Lines:
		for _, l := range v.Lines {
			pipeLine(b, []string{l.Text})
		}
	case present.Notice:
		pipeLine(b, []string{v.Text})
	}
}

func pipeLine(b *strings.Builder, fields []string) {
	for len(fields) > 1 && fields[len(fields)-1] == "" {
		fields = fields[:len(fields)-1]
	}
	b.WriteString(strings.Join(fields, "\t"))
	b.WriteByte('\n')
}

type ttyRenderer struct {
	width int
	theme theme.Theme
}

const (
	margin    = " "
	rowIndent = "   "
	cellGap   = "  "
)

func (r ttyRenderer) Render(w io.Writer, a present.Answer) error {
	_, err := io.WriteString(w, r.document(a.Doc))
	return err
}

func (r ttyRenderer) document(d present.Document) string {
	var b strings.Builder
	r.header(&b, d)
	widths := columnWidths(d.Blocks)
	for _, block := range d.Blocks {
		b.WriteByte('\n')
		r.block(&b, block, widths)
	}
	if len(d.Hints) > 0 {
		b.WriteByte('\n')
		for _, h := range d.Hints {
			b.WriteString(margin + r.theme.Muted.Render("→ "+h) + "\n")
		}
	}
	return b.String()
}

func (r ttyRenderer) header(b *strings.Builder, d present.Document) {
	left := "station"
	styled := r.theme.Title.Render("station")
	if path := commandPath(d.Command); path != "" {
		left += " " + path
		styled += " " + r.theme.Plain.Render(path)
	}
	gap := r.width - 1 - ansi.StringWidth(margin+left) - ansi.StringWidth(d.Context)
	if gap < 1 {
		gap = 1
	}
	line := margin + styled
	if d.Context != "" {
		line += strings.Repeat(" ", gap) + r.theme.Muted.Render(d.Context)
	}
	b.WriteString(line + "\n")
	b.WriteString(margin + r.theme.Subtle.Render(strings.Repeat("─", max(r.width-2, 1))) + "\n")
}

// invariant: a presenter names its command whole ("station tools") and a fixture
// may name it bare ("tools"); the header prints "station" once either way.
func commandPath(command string) string {
	return strings.TrimSpace(strings.TrimPrefix(command, "station"))
}

func (r ttyRenderer) block(b *strings.Builder, block present.Block, widths []int) {
	switch v := block.(type) {
	case present.Section:
		b.WriteString(margin + r.theme.Title.Render(v.Title))
		for _, c := range orderedCounts(v.Counts) {
			b.WriteString(cellGap + r.markStyle(c.Mark).Render(countText(c)))
		}
		b.WriteString("\n")
		for _, inner := range v.Blocks {
			r.block(b, inner, widths)
		}
	case present.Table:
		for _, row := range v.Rows {
			r.row(b, row, widths)
		}
	case present.KeyValues:
		keyWidth := 0
		for _, p := range v.Pairs {
			keyWidth = max(keyWidth, ansi.StringWidth(p.Key))
		}
		for _, p := range v.Pairs {
			key := r.theme.Strong.Render(p.Key) + strings.Repeat(" ", keyWidth-ansi.StringWidth(p.Key))
			b.WriteString(rowIndent + key + cellGap + r.cell(p.Value) + "\n")
		}
	case present.Lines:
		for _, l := range v.Lines {
			b.WriteString(rowIndent + r.cell(l) + "\n")
		}
	case present.Notice:
		b.WriteString(rowIndent + r.toneStyle(v.Tone).Render(v.Text) + "\n")
	}
}

func (r ttyRenderer) row(b *strings.Builder, row present.Row, widths []int) {
	var line strings.Builder
	line.WriteString(rowIndent)
	if g := row.Mark.Glyph(); g != "" {
		line.WriteString(r.markStyle(row.Mark).Render(g) + " ")
	} else {
		line.WriteString(cellGap)
	}
	used := ansi.StringWidth(rowIndent) + 2
	for i, c := range row.Cells {
		if i == len(row.Cells)-1 {
			room := r.width - 1 - used
			text := c.Text
			if ansi.StringWidth(text) > room {
				text = ansi.Truncate(text, max(room, 1), "…")
			}
			line.WriteString(r.cell(present.Cell{Text: text, Tone: c.Tone}))
			break
		}
		pad := widths[i] - ansi.StringWidth(c.Text)
		line.WriteString(r.cell(c) + strings.Repeat(" ", pad) + cellGap)
		used += widths[i] + len(cellGap)
	}
	b.WriteString(strings.TrimRight(line.String(), " ") + "\n")
}

func (r ttyRenderer) cell(c present.Cell) string {
	if c.Text == "" {
		return ""
	}
	return r.toneStyle(c.Tone).Render(c.Text)
}

func (r ttyRenderer) toneStyle(tone present.Tone) lipgloss.Style { return ToneStyle(r.theme, tone) }

func (r ttyRenderer) markStyle(m present.Mark) lipgloss.Style { return MarkStyle(r.theme, m) }

func ToneStyle(t theme.Theme, tone present.Tone) lipgloss.Style {
	switch tone {
	case present.Strong:
		return t.Strong
	case present.Muted:
		return t.Muted
	case present.Accent:
		return t.Accent
	case present.Good:
		return t.Good
	case present.Warn:
		return t.Warn
	case present.Bad:
		return t.Bad
	case present.Plain:
		return t.Plain
	}
	return t.Plain
}

func MarkStyle(t theme.Theme, m present.Mark) lipgloss.Style {
	switch m {
	case present.MarkOK:
		return t.Good
	case present.MarkDegraded:
		return t.Warn
	case present.MarkAbsent:
		return t.Muted
	case present.MarkBroken:
		return t.Bad
	case present.NoMark:
		return t.Plain
	}
	return t.Plain
}

func columnWidths(blocks []present.Block) []int {
	var widths []int
	var walk func(present.Block)
	walk = func(block present.Block) {
		switch v := block.(type) {
		case present.Section:
			for _, inner := range v.Blocks {
				walk(inner)
			}
		case present.Table:
			for _, row := range v.Rows {
				for i, c := range row.Cells {
					if i >= len(widths) {
						widths = append(widths, 0)
					}
					widths[i] = max(widths[i], ansi.StringWidth(c.Text))
				}
			}
		}
	}
	for _, block := range blocks {
		walk(block)
	}
	return widths
}

func orderedCounts(counts []present.Count) []present.Count {
	ordered := make([]present.Count, 0, len(counts))
	for _, m := range []present.Mark{present.MarkOK, present.MarkDegraded, present.MarkAbsent, present.MarkBroken} {
		for _, c := range counts {
			if c.Mark == m && c.N > 0 {
				ordered = append(ordered, c)
			}
		}
	}
	return ordered
}

func countText(c present.Count) string {
	return strconv.Itoa(c.N) + " " + c.Mark.Glyph()
}
