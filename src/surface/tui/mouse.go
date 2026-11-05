package tui

import (
	"time"

	tea "charm.land/bubbletea/v2"

	ui "github.com/jterrazz/studio-station/src/ui/render/tui"
)

// value: two clicks on one cell within this are a double click.
const doubleClick = 400 * time.Millisecond

// value: the last click, for the double click that opens.
type clicked struct {
	x, y int
	at   time.Time
}

// value: the screen's first line of a pane's content: under the header,
// the breadcrumb and the pane's top border.
const paneTop = 3

// invariant: SPEC §9.3: a click selects, a double click opens, the wheel
// scrolls; nothing needs the mouse, and a modal takes no click.
func (s shell) click(m tea.Mouse) (tea.Model, tea.Cmd) {
	if m.Button != tea.MouseLeft || s.sw != nil || s.pal != nil || s.dialog != nil || s.acts.modal != nil || s.sheet {
		return s, nil
	}
	double := s.last.x == m.X && s.last.y == m.Y && time.Since(s.last.at) < doubleClick
	s.last = clicked{x: m.X, y: m.Y, at: time.Now()}
	line := m.Y - paneTop
	if line < 0 {
		return s, nil
	}
	if s.sidebarShown() && m.X < ui.SidebarWidth {
		entries := s.entries()
		if line >= len(entries) || !entries[line].selectable() {
			return s, nil
		}
		s.focus, s.side = ui.PaneSidebar, line
		e := entries[line]
		switch {
		case e.isSection:
			return s.show(e.sec)
		case double:
			return s.open(e.machine)
		}
		return s, nil
	}
	if s.matrix() || s.table() && !double {
		s.focus = ui.PaneMaster
		return s, nil
	}
	key := s.key(s.current(), s.section())
	l, ok := s.list(key).At(s.frame(), s.masterWidth(), max(s.height-5, 1), line)
	s.focus = ui.PaneMaster
	if !ok {
		return s, nil
	}
	s.lists = with(s.lists, key, l)
	s.acts.shown = with(s.acts.shown, key, shown{})
	if double {
		return s.master("enter")
	}
	return s, nil
}

func (s shell) wheel(m tea.Mouse) (tea.Model, tea.Cmd) {
	if s.sw != nil || s.pal != nil || s.dialog != nil || s.acts.modal != nil {
		return s, nil
	}
	key := s.key(s.current(), s.section())
	step := map[tea.MouseButton]int{tea.MouseWheelUp: -1, tea.MouseWheelDown: 1}[m.Button]
	if step != 0 {
		s.lists = with(s.lists, key, s.list(key).Move(step))
	}
	return s, nil
}
