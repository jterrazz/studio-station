package tui

import (
	"slices"

	ui "github.com/jterrazz/studio-station/src/ui/render/tui"
)

func bind(key, label string) ui.Binding { return ui.Binding{Key: key, Label: label} }

func (s shell) paneKeys() []ui.Binding {
	switch {
	case s.acts.modal != nil:
		return s.acts.modal.keys()
	case s.focus == ui.PaneProgress:
		if t, ok := s.waiting(); ok {
			return t.keys()
		}
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
	if s.ticks() {
		keys = append(keys, bind("space", "tick"))
	}
	acts := s.actionKeys()
	_, folded := s.list(s.key(s.current(), s.section())).Unfold()
	if folded {
		acts = slices.DeleteFunc(acts, func(b ui.Binding) bool { return b.Key == "enter" })
	}
	keys = append(keys, acts...)
	switch {
	case folded:
		keys = append(keys, bind("enter", "unfold"))
	case s.table():
		keys = append(keys, bind("enter", "open"))
	case slices.ContainsFunc(acts, func(b ui.Binding) bool { return b.Key == "enter" }):
	case s.listed():
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

func (s shell) footer() []ui.Binding {
	keys := s.paneKeys()
	if s.filtering || s.acts.modal != nil || s.focus == ui.PaneProgress {
		return keys
	}
	return append(keys, bind("tab", "next pane"), bind("[ ]", "section"))
}

func (s shell) keySheet() *ui.Sheet {
	pane := map[ui.Pane]string{ui.PaneSidebar: "Sidebar", ui.PaneMaster: s.section().Title(), ui.PaneDetail: "Detail"}[s.focus]
	return &ui.Sheet{Title: "Keys" + ui.Sep + pane, Groups: []ui.SheetGroup{
		{Title: pane, Keys: s.paneKeys()},
		{Title: "Also", Keys: s.aliases()},
		{Title: "Everywhere", Keys: append(s.everywhere(), bind("?", "every key"))},
	}}
}
