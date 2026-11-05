package tui

import (
	"slices"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/jterrazz/studio-station/src/page"
	"github.com/jterrazz/studio-station/src/ui/atom"
	"github.com/jterrazz/studio-station/src/ui/molecule"
	"github.com/jterrazz/studio-station/src/ui/organism"
	"github.com/jterrazz/studio-station/src/ui/token"
)

type Pane int

const (
	PaneSidebar Pane = iota
	PaneMaster
	PaneDetail
	PaneProgress
)

const (
	SidebarWidth = 24
	DetailWidth  = 36
	ThreePanes   = 110
	SidebarFrom  = 80
	LeastWidth   = 60
	LeastHeight  = 16
)

type Header struct {
	Mark  page.Mark
	Parts []string
	// value: the parts a narrow terminal leaves out first, by index.
	Drop []int
}

type Entry struct {
	Label       string
	Mark        page.Mark
	Badge       string
	Tone        page.Mark
	Depth       int
	Branch      bool
	Last        bool
	AllMachines bool
	Heading     bool
	Current     bool
}

type Panel struct {
	Title string
	Lines []string
}

type Sheet struct {
	Title  string
	Groups []SheetGroup
}

type SheetGroup struct {
	Title string
	Keys  []organism.Binding
}

type Shell struct {
	Width, Height int
	Dark, NoColor bool
	Header        Header
	Crumbs        []string
	Sidebar       []Entry
	Cursor        int
	ShowSidebar   bool
	Master        Panel
	Detail        *Panel
	// value: the fixed panes of a page section at the right of the master
	// pane, top to bottom, when there is no detail (a server's Health and
	// Ports).
	Beside  []Panel
	Overlay bool
	Focus   Pane
	Footer  []organism.Binding
	Sheet   *Sheet
	// value: a form or a screen's question, over the panes until it is
	// answered.
	Dialog *Panel
	// value: the Progress strip, above the footer, only while not empty.
	Progress []Task
	// value: the question put, drawn over the master pane.
	Modal *Panel
}

func Fits(width, height int) bool { return width >= LeastWidth && height >= LeastHeight }

func TooSmall(width, height int) string {
	return "station needs " + strconv.Itoa(LeastWidth) + " × " + strconv.Itoa(LeastHeight) + "; this terminal is " +
		strconv.Itoa(width) + " × " + strconv.Itoa(height) + token.Sep + "station overview prints it"
}

func (s Shell) Render() string {
	st := token.New(s.Dark)
	if !Fits(s.Width, s.Height) {
		return lipgloss.NewStyle().Width(max(s.Width, 1)).Render(TooSmall(s.Width, s.Height))
	}
	strip := s.progressLines(st)
	room := max(s.Height-3-len(strip), 3)
	lines := make([]string, 0, room+3+len(strip))
	lines = append(lines, s.header(st), " "+st.Muted.Render(strings.Join(s.Crumbs, token.Crumb)))
	var body []string
	switch {
	case s.Sheet != nil:
		body = s.box(st, s.Sheet.Title, s.sheetLines(st), s.Width, room, true)
	case s.Dialog != nil:
		body = s.box(st, s.Dialog.Title, s.Dialog.Lines, s.Width, room, true)
	default:
		body = s.panes(st, room)
	}
	lines = append(lines, body...)
	lines = append(lines, strip...)
	lines = append(lines, organism.Footer(st, s.Footer, false, s.Width))
	return strings.Join(lines, "\n")
}

func (s Shell) header(st token.Styles) string {
	parts := slices.Clone(s.Header.Parts)
	line := func() string {
		var kept []string
		for _, p := range parts {
			if p != "" {
				kept = append(kept, p)
			}
		}
		return st.Title.Render("station") + "  " + atom.Mark(st, s.Header.Mark) + st.Text.Render(strings.Join(kept, token.Sep))
	}
	for _, i := range s.Header.Drop {
		if atom.Width(line()) <= s.Width {
			break
		}
		if i < len(parts) {
			parts[i] = ""
		}
	}
	return fit(line(), s.Width)
}

func ErrorBlock(e page.Error, dark bool) []string {
	lines := molecule.ErrorLines(token.New(dark), e)
	for i := range lines {
		lines[i] = " " + lines[i]
	}
	return lines
}

func (s Shell) panes(st token.Styles, room int) []string {
	width := s.Width
	var left []string
	if s.ShowSidebar {
		left = s.box(st, "", s.sidebarLines(st), SidebarWidth, room, s.Focus == PaneSidebar)
		width -= SidebarWidth
	}
	var right []string
	masterWidth := width
	if s.Detail != nil && !s.Overlay {
		masterWidth = width - DetailWidth
		right = s.box(st, s.Detail.Title, s.Detail.Lines, DetailWidth, room, s.Focus == PaneDetail)
	}
	if s.Detail == nil && len(s.Beside) > 0 {
		masterWidth = width - DetailWidth
		right = s.stack(st, room)
	}
	master := s.box(st, s.Master.Title, s.Master.Lines, masterWidth, room, s.Focus == PaneMaster)
	if s.Modal != nil {
		master = s.box(st, s.Modal.Title, s.Modal.Lines, masterWidth, room, true)
	}
	if s.Detail != nil && s.Overlay {
		over := s.box(st, s.Detail.Title, s.Detail.Lines, min(DetailWidth, masterWidth), room, s.Focus == PaneDetail)
		for i := range master {
			master[i] = lipgloss.NewStyle().MaxWidth(masterWidth-min(DetailWidth, masterWidth)).Render(master[i]) + over[i]
		}
	}
	out := make([]string, room)
	for i := range room {
		line := ""
		if left != nil {
			line += left[i]
		}
		line += master[i]
		if right != nil {
			line += right[i]
		}
		out[i] = line
	}
	return out
}

// invariant: each fixed pane takes the lines it has, the last one the room
// left, so the column is as tall as the master pane.
func (s Shell) stack(st token.Styles, room int) []string {
	out := make([]string, 0, room)
	for i, p := range s.Beside {
		height := min(len(p.Lines)+2, room-len(out))
		if i == len(s.Beside)-1 {
			height = room - len(out)
		}
		if height < 2 {
			break
		}
		out = append(out, s.box(st, p.Title, p.Lines, DetailWidth, height, false)...)
	}
	for len(out) < room {
		out = append(out, atom.Spaces(DetailWidth))
	}
	return out
}

func (s Shell) sidebarLines(st token.Styles) []string {
	inner := SidebarWidth - 2
	out := make([]string, 0, len(s.Sidebar))
	for i, e := range s.Sidebar {
		if e.Heading {
			text := st.Muted.Render(e.Label)
			if e.Depth == 0 {
				text = st.Strong.Render(e.Label)
			}
			out = append(out, fit(" "+atom.Spaces(e.Depth)+text, inner))
			continue
		}
		lead := " "
		if e.Current {
			lead = token.Cursor
		}
		left := lead + atom.Spaces(e.Depth)
		switch {
		case e.Branch && e.Last:
			left += token.SideEnd
		case e.Branch:
			left += token.SideMid
		case e.AllMachines:
			left += token.Fleet + " "
		}
		if e.Mark != page.Unjudged {
			left += atom.Mark(st, e.Mark)
		}
		left += e.Label
		badge := ""
		if e.Badge != "" {
			badge = atom.Tone(st, e.Tone).Render(e.Badge) + " "
		}
		line := atom.Pad(lipgloss.NewStyle().MaxWidth(inner-atom.Width(badge)).Render(left), inner-atom.Width(badge)) + badge
		if i == s.Cursor && s.Focus == PaneSidebar {
			line = s.cursor(st, line, inner)
		}
		out = append(out, line)
	}
	return out
}

func (s Shell) cursor(st token.Styles, line string, width int) string {
	if s.NoColor {
		return lipgloss.NewStyle().Reverse(true).Render(atom.Pad(line, width))
	}
	return st.Selection.Width(width).Render(line)
}

func (s Shell) CursorLine(line string, width int) string {
	return s.cursor(token.New(s.Dark), line, width)
}

func (s Shell) box(st token.Styles, title string, body []string, width, height int, focused bool) []string {
	b := lipgloss.RoundedBorder()
	tone := st.Subtle
	if focused {
		tone = st.Accent
		if s.NoColor {
			b = lipgloss.ThickBorder()
		}
	}
	inner := max(width-2, 1)
	top := b.TopLeft + b.Top
	if title != "" {
		top += " " + title + " "
	}
	top = atom.End(top, width-1)
	top += atom.Repeat(b.Top, width-1-atom.Width(top)) + b.TopRight
	out := make([]string, 0, height)
	out = append(out, tone.Render(top))
	for i := range max(height-2, 0) {
		line := ""
		if i < len(body) {
			line = body[i]
		}
		out = append(out, tone.Render(b.Left)+fit(line, inner)+tone.Render(b.Right))
	}
	out = append(out, tone.Render(b.BottomLeft+atom.Repeat(b.Bottom, inner)+b.BottomRight))
	return out
}

func (s Shell) sheetLines(st token.Styles) []string {
	var out []string
	for _, g := range s.Sheet.Groups {
		if len(g.Keys) == 0 {
			continue
		}
		keyWidth := 0
		for _, k := range g.Keys {
			keyWidth = max(keyWidth, atom.Width(atom.KeyName(k.Key)))
		}
		out = append(out, " "+st.Title.Render(strings.ToUpper(g.Title)))
		for _, k := range g.Keys {
			out = append(out, "   "+st.Accent.Render(atom.Pad(atom.KeyName(k.Key), keyWidth))+atom.Spaces(token.Gap)+st.Text.Render(k.Label))
		}
		out = append(out, "")
	}
	return append(out, " "+st.Muted.Render("any key closes"))
}

func fit(line string, width int) string {
	return atom.Pad(lipgloss.NewStyle().MaxWidth(width).Render(line), width)
}

func Plain(v page.Value) string { return atom.Plain(v) }

func Glyph(m page.Mark) string { return atom.Glyph(m) }

type Binding = organism.Binding

const (
	Sep   = token.Sep
	Crumb = token.Crumb
)
