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
	if s.Joined {
		return atom.Spaces(token.Margin) + st.Title.Render(atom.Show(page.List(append([]page.Value{page.Text(s.Title)}, s.Caption...)...)))
	}
	// invariant: an open section's title counts nothing: its rows are there
	// to read (vision 07 "Updates"); a folded one counts them on its line.
	line := atom.Spaces(token.Margin) + st.Title.Render(s.Title)
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
	right := molecule.Counts(st, s.Counts)
	if right == "" {
		return left
	}
	return left + atom.Spaces(max(width-1-atom.Width(left)-atom.Width(right), token.Gap)) + right
}

func Rows(st token.Styles, s page.Section, width int) []string {
	body := (Grid{Styles: st, Section: s, Width: width, Medium: molecule.TTY}).Lines()
	lines := make([]string, 0, len(body))
	for _, l := range body {
		lines = append(lines, l.Text)
	}
	return lines
}

func Section(st token.Styles, s page.Section, width int) []string {
	body := Rows(st, s, width)
	lines := make([]string, 0, len(body)+1)
	// invariant: a section with no title and no caption reads on as its
	// rows (an Overview's lines under its Needs you, Awake's Macs), with no
	// blank head, whatever it counts.
	if s.Title != "" || len(s.Caption) > 0 {
		lines = append(lines, Title(st, s))
	}
	return append(lines, body...)
}

func AutoFolded(s page.Section) bool {
	switch s.Fold {
	case page.Folded:
		return true
	case page.Open, page.Brief:
		return false
	case page.Auto, page.Calm:
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
