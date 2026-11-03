package organism

import (
	"github.com/jterrazz/studio-station/src/page"
	"github.com/jterrazz/studio-station/src/ui/atom"
	"github.com/jterrazz/studio-station/src/ui/molecule"
	"github.com/jterrazz/studio-station/src/ui/token"
)

func Caption(st token.Styles, s page.Section) string {
	if len(s.Caption) == 0 {
		return ""
	}
	parts := make([]page.Value, 0, len(s.Caption))
	parts = append(parts, s.Caption...)
	return st.Muted.Render(atom.Show(page.List(parts...)))
}

func Title(st token.Styles, s page.Section) string {
	line := atom.Spaces(token.Margin) + st.Title.Render(s.Title)
	if s.Role != page.Banner {
		if counts := molecule.Counts(st, s.Counts); counts != "" {
			line += atom.Spaces(token.Gap) + counts
		}
	}
	if c := Caption(st, s); c != "" {
		line += atom.Spaces(token.Gap) + c
	}
	return line
}

func FoldTitle(st token.Styles, s page.Section, folded bool, width int) string {
	sign := token.FoldOpen
	if folded {
		sign = token.FoldShut
	}
	left := atom.Spaces(token.Margin) + st.Muted.Render(sign) + " " + st.Title.Render(s.Title)
	if c := Caption(st, s); c != "" {
		left += atom.Spaces(token.Gap) + c
	}
	right := ""
	if s.Role != page.Banner {
		right = molecule.Counts(st, s.Counts)
	}
	if right == "" {
		return left
	}
	return left + atom.Spaces(max(width-1-atom.Width(left)-atom.Width(right), token.Gap)) + right
}

func Section(st token.Styles, s page.Section, width int) []string {
	body := (Grid{Styles: st, Section: s, Width: width, Medium: molecule.TTY}).Lines()
	lines := make([]string, 0, len(body)+1)
	lines = append(lines, Title(st, s))
	for _, l := range body {
		lines = append(lines, l.Text)
	}
	return lines
}

func AutoFolded(s page.Section) bool {
	switch s.Fold {
	case page.Folded:
		return true
	case page.Open:
		return false
	case page.Auto:
		rows := 0
		for _, t := range s.Counts {
			if t.Mark != page.OK && t.Mark != page.Absent {
				return false
			}
			rows += t.N
		}
		return rows > 0
	}
	return false
}
