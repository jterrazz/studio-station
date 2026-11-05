package template

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/jterrazz/studio-station/src/page"
	"github.com/jterrazz/studio-station/src/ui/atom"
	"github.com/jterrazz/studio-station/src/ui/molecule"
	"github.com/jterrazz/studio-station/src/ui/organism"
	"github.com/jterrazz/studio-station/src/ui/token"
)

func Document(st token.Styles, p page.Page, width int) []string {
	// invariant: the CLI page starts at its own first line, at the terminal's
	// first column (vision 07): no header, no rule, no margin; the organisms lay
	// out at the margin the TUI keeps, and the document takes it off.
	lines := body(st, p, width+token.Margin)
	for i, l := range lines {
		lines[i] = unindent(l)
	}
	for len(lines) > 0 && lines[0] == "" {
		lines = lines[1:]
	}
	return lines
}

func body(st token.Styles, p page.Page, width int) []string {
	var lines []string
	if p.Error != nil {
		lines = append(lines, "")
		for _, l := range molecule.ErrorLines(st, *p.Error) {
			lines = append(lines, atom.Spaces(token.Margin)+l)
		}
		return lines
	}
	first := firstLine(p.Sections)
	fold := foldWidth(p.Sections)
	for i, s := range p.Sections {
		if i == first+1 {
			lines = append(lines, "")
		}
		switch {
		case i == first && len(s.Items) == 1 && s.Items[0].Row != nil && !s.Joined && atom.Width(summed(st, *s.Items[0].Row)) < width-token.Margin:
			lines = append(lines, summed(st, *s.Items[0].Row))
			if h := s.Items[0].Row.Hint; h != nil && !elsewhere(p, i, h.Command) {
				lines = append(lines, atom.Spaces(token.Margin+token.Indent)+molecule.PageHint(st, *h, 0))
			}
		case i == first && s.Joined && len(s.Items) > 0:
			lines = append(lines, organism.Title(st, s), "")
			lines = append(lines, organism.Rows(st, s, width)...)
		case s.Fold == page.Folded || s.Fold == page.Brief:
			lines = append(lines, folded(st, s, fold, width))
		default:
			lines = append(lines, organism.Section(st, s, width)...)
		}
	}
	if len(p.Hints) > 0 {
		lines = append(lines, "")
		at := 0
		for _, h := range p.Hints {
			if h.Text != "" {
				at = max(at, atom.Width(h.Command))
			}
		}
		for _, h := range p.Hints {
			lines = append(lines, atom.Spaces(token.Margin)+molecule.PageHint(st, h, at))
		}
	}
	return lines
}

// value: the section that sums the page up (SPEC §5: the first line), after
// the On line; the one blank line of a page's body follows it, the others
// read on without one (vision 07). -1 when the page has none.
func firstLine(sections []page.Section) int {
	for i, s := range sections {
		if s.ID == page.OnSection {
			continue
		}
		if s.Role == page.Banner || s.Role == page.Summary {
			return i
		}
		return -1
	}
	return -1
}

// value: a page summed up by one machine's row reads as its first line,
// `This Mac · 108 apps for this Mac · 2 updates` (vision 07 "station apps"),
// when the line holds it whole; a longer one stays its section.
func summed(st token.Styles, r page.Row) string {
	parts := []page.Value{page.Text(r.Label)}
	parts = append(parts, r.Cells...)
	if !r.Detail.IsNone() {
		parts = append(parts, r.Detail)
	}
	return atom.Spaces(token.Margin) + st.Title.Render(atom.Show(page.List(parts...)))
}

// value: whether a hint the first line leaves out is on another row of the
// page, or one of its next steps.
func elsewhere(p page.Page, skip int, command string) bool {
	for _, h := range p.Hints {
		if h.Command == command {
			return true
		}
	}
	var walk func(items []page.Item) bool
	walk = func(items []page.Item) bool {
		for _, it := range items {
			switch {
			case it.Row != nil && it.Row.Hint != nil && it.Row.Hint.Command == command:
				return true
			case it.Group != nil && (it.Group.Head.Hint != nil && it.Group.Head.Hint.Command == command || walk(it.Group.Items)):
				return true
			}
		}
		return false
	}
	for i, s := range p.Sections {
		if i != skip && walk(s.Items) {
			return true
		}
	}
	return false
}

var styled = regexp.MustCompile(`^((?:\x1b\[[0-9;:]*m)*) `)

// value: a line with the margin the organisms keep taken off, past the
// styles a line may open with.
func unindent(l string) string {
	for range token.Margin {
		l = styled.ReplaceAllString(l, "$1")
	}
	return l
}

func foldWidth(sections []page.Section) int {
	n := 0
	for _, s := range sections {
		if s.Fold == page.Folded || s.Fold == page.Brief {
			n = max(n, atom.Width(s.Title))
		}
	}
	return n
}

// invariant: a folded section is one line: its title, its count by mark
// (`○ 34`), then as many of its rows' names as the line holds (vision 07:
// "Not installed   ○ 34 · yazi, dust, eza, …"); --all unfolds it.
func folded(st token.Styles, s page.Section, at, width int) string {
	line := atom.Spaces(token.Margin) + st.Title.Render(atom.Pad(s.Title, at))
	var counts []string
	for _, t := range s.Counts {
		if t.N > 0 {
			counts = append(counts, atom.Tone(st, t.Mark).Render(atom.Glyph(t.Mark))+" "+st.Text.Render(strconv.Itoa(t.N)))
		}
	}
	if len(counts) == 0 {
		return strings.TrimRight(line, " ")
	}
	line += atom.Spaces(token.Gap) + strings.Join(counts, atom.Spaces(token.Gap))
	var names []string
	for _, it := range s.Items {
		if it.Row != nil {
			names = append(names, it.Row.Label)
		}
	}
	if len(names) == 0 {
		return line
	}
	room := width - 1 - atom.Width(line) - atom.Width(token.Sep)
	list := ""
	for i, n := range names {
		next := list
		if i > 0 {
			next += ", "
		}
		next += n
		more := ""
		if i < len(names)-1 {
			more = ", " + token.Ellipsis
		}
		if atom.Width(next+more) > room {
			if list == "" {
				return line
			}
			list += ", " + token.Ellipsis
			break
		}
		list = next
	}
	return line + st.Muted.Render(token.Sep+list)
}
