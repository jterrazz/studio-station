package kit

import (
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/jterrazz/studio-station/src/output"
	"github.com/jterrazz/studio-station/src/present"
	"github.com/jterrazz/studio-station/src/theme"
)

const (
	margin    = " "
	rowIndent = "   "
	cellGap   = "  "
)

func Cell(t theme.Theme, c present.Cell) string {
	if c.Text == "" {
		return ""
	}
	return output.ToneStyle(t, c.Tone).Render(c.Text)
}

func MarkText(t theme.Theme, m present.Mark) string {
	if g := m.Glyph(); g != "" {
		return output.MarkStyle(t, m).Render(g)
	}
	return ""
}

func CountsText(t theme.Theme, counts []present.Count) string {
	parts := make([]string, 0, len(counts))
	for _, c := range counts {
		if c.N > 0 {
			parts = append(parts, output.MarkStyle(t, c.Mark).Render(strconv.Itoa(c.N)+" "+c.Mark.Glyph()))
		}
	}
	return strings.Join(parts, cellGap)
}

func StatusLine(t theme.Theme, s Status, width int) string {
	if s.Text == "" {
		return ""
	}
	line := margin
	if g := s.Mark.Glyph(); g != "" {
		line += MarkText(t, s.Mark) + " "
	}
	line += t.Plain.Render(Truncate(s.Text, max(width-6, 1)))
	if s.Hint != "" {
		line += cellGap + t.Muted.Render(s.Hint)
	}
	return line
}

func Truncate(s string, width int) string {
	if lipgloss.Width(s) <= width {
		return s
	}
	if width <= 1 {
		return "…"
	}
	runes := []rune(s)
	for len(runes) > 0 && lipgloss.Width(string(runes))+1 > width {
		runes = runes[:len(runes)-1]
	}
	return string(runes) + "…"
}

func Frame(t theme.Theme, title, body string, width int) string {
	box := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(t.Tokens.Surface).
		Padding(0, 1).
		Width(max(width-2, 10))
	head := ""
	if title != "" {
		head = t.Title.Render(title) + "\n"
	}
	return lipgloss.NewStyle().MarginLeft(1).Render(box.Render(head + strings.TrimRight(body, "\n")))
}
