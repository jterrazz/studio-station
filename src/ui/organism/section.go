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

// value: a banner prints no counts on a terminal (its page is that one list),
// but a tree's heads are the verdict a reader looks for first (the Fleet), so
// a banner made of judged groups counts them like any section (§3.3).
func titleCounts(s page.Section) bool {
	if s.Role != page.Banner {
		return true
	}
	for _, it := range s.Items {
		if it.Group != nil && !it.Group.Heading() {
			return true
		}
	}
	return false
}

func Title(st token.Styles, s page.Section) string {
	if s.Joined {
		return atom.Spaces(token.Margin) + st.Title.Render(atom.Show(page.List(append([]page.Value{page.Text(s.Title)}, s.Caption...)...)))
	}
	line := atom.Spaces(token.Margin) + st.Title.Render(s.Title)
	if titleCounts(s) {
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
	right := molecule.Counts(st, s.Counts)
	if right == "" {
		return left
	}
	return left + atom.Spaces(max(width-1-atom.Width(left)-atom.Width(right), token.Gap)) + right
}

func Section(st token.Styles, s page.Section, width int) []string {
	body := (Grid{Styles: st, Section: s, Width: width, Medium: molecule.TTY}).Lines()
	lines := make([]string, 0, len(body)+1)
	// invariant: a section with no title and no caption reads on as its
	// rows (an Overview's lines under its Needs you), with no blank head.
	if s.Title != "" || len(s.Caption) > 0 || len(s.Counts) > 0 {
		lines = append(lines, Title(st, s))
	}
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
