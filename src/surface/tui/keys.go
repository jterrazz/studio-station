package tui

import (
	ui "github.com/jterrazz/studio-station/src/ui/render/tui"
)

func bind(key, label string) ui.Binding { return ui.Binding{Key: key, Label: label} }

func (s shell) paneKeys() []ui.Binding {
	switch {
	case s.filtering:
		return []ui.Binding{bind("type", "filter"), bind("enter", "keep"), bind("esc", "clear")}
	case s.focus == ui.PaneSidebar:
		keys := []ui.Binding{bind("up/down", "move"), bind("enter", "open"), bind("right", "list")}
		if s.current().kind != "" {
			keys = append(keys, bind("esc", "up a level"))
		}
		return keys
	case s.focus == ui.PaneDetail:
		return []ui.Binding{bind("left", "list"), bind("esc", "back")}
	}
	keys := []ui.Binding{bind("up/down", "move")}
	if _, folded := s.list(s.key(s.current(), s.section())).Unfold(); folded {
		keys = append(keys, bind("enter", "unfold"))
	} else if s.table() {
		keys = append(keys, bind("enter", "open"))
	} else if s.detailed() {
		keys = append(keys, bind("enter", "detail"))
	}
	if s.listed() {
		keys = append(keys, bind("/", "filter"))
	}
	if s.sidebarShown() {
		keys = append(keys, bind("left", "sidebar"))
	}
	if s.current().kind != "" {
		keys = append(keys, bind("esc", "up a level"))
	}
	return keys
}

func (s shell) everywhere() []ui.Binding {
	if s.filtering {
		return nil
	}
	return []ui.Binding{bind("tab", "next pane"), bind("shift+tab", "previous pane"), bind("[ ]", "section"),
		bind("ctrl+r", "read again"), bind("q", "quit")}
}

func (s shell) aliases() []ui.Binding {
	// invariant: every key the shell answers is on the sheet, its aliases too.
	keys := []ui.Binding{bind("k/j", "move up/down"), bind("h/l", "left/right")}
	if s.focus == ui.PaneMaster && s.listed() {
		keys = append(keys, bind("pgup/pgdown", "move ten rows"))
	}
	return keys
}

func (s shell) screenKeys() []ui.Binding {
	if sc := s.screen(); sc != nil && s.read() && s.focus != ui.PaneSidebar {
		return sc.Keys(s.view())
	}
	return nil
}

func (s shell) footer() []ui.Binding {
	if s.dialog != nil {
		return nil
	}
	keys := append(s.screenKeys(), s.paneKeys()...)
	if s.filtering {
		return keys
	}
	return append(keys, bind("tab", "next pane"), bind("[ ]", "section"))
}

func (s shell) keySheet() *ui.Sheet {
	pane := map[ui.Pane]string{ui.PaneSidebar: "Sidebar", ui.PaneMaster: s.section().Title(), ui.PaneDetail: "Detail"}[s.focus]
	return &ui.Sheet{Title: "Keys" + ui.Sep + pane, Groups: []ui.SheetGroup{
		{Title: pane, Keys: append(s.screenKeys(), s.paneKeys()...)},
		{Title: "Also", Keys: s.aliases()},
		{Title: "Everywhere", Keys: append(s.everywhere(), bind("?", "every key"))},
	}}
}
