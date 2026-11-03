package tui

import (
	"slices"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"

	"github.com/jterrazz/studio-station/src/page"
	"github.com/jterrazz/studio-station/src/ui/atom"
	"github.com/jterrazz/studio-station/src/ui/molecule"
	"github.com/jterrazz/studio-station/src/ui/organism"
	"github.com/jterrazz/studio-station/src/ui/token"
)

type Focus int

const (
	Normal Focus = iota
	Filter
	Detail
	Modal
	Sheet
)

type Event struct {
	Action page.Action
	Row    string
}

type Browser struct {
	Page   page.Page
	Noun   string
	Tab    string
	Cursor string
	Folds  map[string]bool
	styles token.Styles
	filter textinput.Model
	focus  Focus
	detail bool
	ask    *ask
	form   *form
}

type form struct {
	f      *huh.Form
	read   func() map[string]string
	submit func(values map[string]string) tea.Cmd
}

type ask struct {
	confirm page.Confirm
	yes     bool
	then    tea.Cmd
}

func New(tab, noun string) Browser {
	f := textinput.New()
	f.Prompt = "/ "
	f.Placeholder = "filter"
	return Browser{Tab: tab, Noun: noun, Folds: map[string]bool{}, styles: token.New(true), filter: f}
}

func (b Browser) Themed(dark bool) Browser {
	b.styles = token.New(dark)
	return b
}

func (b Browser) Focus() Focus {
	if b.ask != nil || b.form != nil {
		return Modal
	}
	return b.focus
}

func (b Browser) Show(p page.Page) Browser {
	// invariant: a section's fold is decided once its rows have answered, so a
	// frame of pending rows never folds a section the full reading leaves open.
	for _, s := range p.Sections {
		if _, seen := b.Folds[s.ID]; seen || pendingIn(s) {
			continue
		}
		b.Folds[s.ID] = organism.AutoFolded(s)
	}
	b.Page = p
	if b.Cursor == "" || !slices.Contains(b.keys(), b.Cursor) {
		keys := b.keys()
		if b.Cursor != "" {
			section, _, _ := strings.Cut(b.Cursor, "/")
			if slices.Contains(keys, section) {
				b.Cursor = section
				return b
			}
		}
		b.Cursor = ""
		if len(keys) > 0 {
			b.Cursor = keys[0]
		}
	}
	return b
}

func pendingIn(s page.Section) bool {
	for _, t := range s.Counts {
		if t.Mark == page.Awaiting {
			return true
		}
	}
	return false
}

func (b Browser) Current() (section page.Section, row page.Row, ok bool) {
	sid, rid, isRow := strings.Cut(b.Cursor, "/")
	for _, s := range b.Page.Sections {
		if s.ID != sid {
			continue
		}
		if !isRow {
			return s, page.Row{}, false
		}
		if r, found := findRow(s.Items, rid); found {
			return s, r, true
		}
	}
	return page.Section{}, page.Row{}, false
}

func findRow(items []page.Item, id string) (page.Row, bool) {
	for _, it := range items {
		switch {
		case it.Row != nil && it.Row.ID == id:
			return *it.Row, true
		case it.Group != nil && it.Group.Head.ID == id:
			return it.Group.Head, true
		case it.Group != nil:
			if r, ok := findRow(it.Group.Items, id); ok {
				return r, true
			}
		}
	}
	return page.Row{}, false
}

func (b Browser) Ask(c page.Confirm, then tea.Cmd) Browser {
	b.ask = &ask{confirm: c, yes: !c.Danger, then: then}
	return b
}

func (b Browser) Fill(f page.Form, submit func(values map[string]string) tea.Cmd) (Browser, tea.Cmd) {
	fields, read := organism.Fields(f)
	if f.Title != "" {
		fields = append([]huh.Field{huh.NewNote().Title(f.Title)}, fields...)
	}
	h := huh.NewForm(huh.NewGroup(fields...)).WithTheme(huh.ThemeFunc(organism.FormStyles)).WithShowHelp(false).WithWidth(token.ModalWidth - 4)
	b.form = &form{f: h, read: read, submit: submit}
	return b, h.Init()
}

func (b Browser) filling(msg tea.Msg) (Browser, tea.Cmd, *Event) {
	if k, ok := msg.(tea.KeyPressMsg); ok && (k.String() == "esc" || k.String() == "ctrl+c") {
		b.form = nil
		return b, nil, nil
	}
	model, cmd := b.form.f.Update(msg)
	if h, ok := model.(*huh.Form); ok {
		b.form.f = h
	}
	switch b.form.f.State {
	case huh.StateCompleted:
		done := b.form
		b.form = nil
		return b, done.submit(done.read()), nil
	case huh.StateAborted:
		b.form = nil
		return b, nil, nil
	case huh.StateNormal:
	}
	return b, cmd, nil
}

func (b Browser) Update(msg tea.Msg) (Browser, tea.Cmd, *Event) {
	if bg, ok := msg.(tea.BackgroundColorMsg); ok {
		return b.Themed(bg.IsDark()), nil, nil
	}
	if b.form != nil {
		return b.filling(msg)
	}
	k, ok := msg.(tea.KeyPressMsg)
	if !ok {
		if b.focus == Filter {
			var cmd tea.Cmd
			b.filter, cmd = b.filter.Update(msg)
			return b, cmd, nil
		}
		return b, nil, nil
	}
	key := k.String()
	switch b.Focus() {
	case Modal:
		return b.modal(key)
	case Sheet:
		b.focus = Normal
		return b, nil, nil
	case Filter:
		return b.typing(k)
	case Normal, Detail:
	}
	switch key {
	case "up", "k":
		b = b.move(-1)
	case "down", "j":
		b = b.move(1)
	case "g", "home":
		b = b.edge(false)
	case "G", "end":
		b = b.edge(true)
	case "pgup":
		b = b.move(-10)
	case "pgdown":
		b = b.move(10)
	case "space":
		b = b.fold()
	case "/":
		b.focus = Filter
		b.detail = false
		return b, b.filter.Focus(), nil
	case "?":
		b.focus = Sheet
	case "enter":
		b.detail = !b.detail
		if b.detail {
			b.focus = Detail
		} else {
			b.focus = Normal
		}
	case "esc":
		b.detail = false
		b.focus = Normal
	default:
		if a, row, found := b.action(key); found {
			return b, nil, &Event{Action: a, Row: row}
		}
	}
	return b, nil, nil
}

func (b Browser) action(key string) (page.Action, string, bool) {
	_, r, isRow := b.Current()
	for _, a := range b.Page.Actions {
		if a.Key != key {
			continue
		}
		if a.On == page.OnPage {
			return a, "", true
		}
		if isRow && (len(a.Rows) == 0 || slices.Contains(a.Rows, r.ID)) {
			return a, r.ID, true
		}
		return a, "", false
	}
	return page.Action{}, "", false
}

func (b Browser) modal(key string) (Browser, tea.Cmd, *Event) {
	a := b.ask
	switch key {
	case "y":
		b.ask = nil
		return b, a.then, nil
	case "n", "esc", "ctrl+c":
		b.ask = nil
	case "left", "right", "tab", "h", "l":
		next := *a
		next.yes = !a.yes
		b.ask = &next
	case "enter":
		b.ask = nil
		if a.yes {
			return b, a.then, nil
		}
	}
	return b, nil, nil
}

func (b Browser) typing(k tea.KeyPressMsg) (Browser, tea.Cmd, *Event) {
	switch k.String() {
	case "esc":
		b.filter.Reset()
		b.filter.Blur()
		b.focus = Normal
	case "enter":
		b.filter.Blur()
		b.focus = Normal
	default:
		var cmd tea.Cmd
		b.filter, cmd = b.filter.Update(k)
		if keys := b.keys(); !slices.Contains(keys, b.Cursor) {
			b.Cursor = ""
			if len(keys) > 0 {
				b.Cursor = keys[0]
			}
		}
		return b, cmd, nil
	}
	return b, nil, nil
}

func (b Browser) move(n int) Browser {
	keys := b.keys()
	if len(keys) == 0 {
		return b
	}
	i := max(slices.Index(keys, b.Cursor), 0)
	b.Cursor = keys[min(max(i+n, 0), len(keys)-1)]
	return b
}

func (b Browser) edge(last bool) Browser {
	keys := b.keys()
	if len(keys) == 0 {
		return b
	}
	b.Cursor = keys[0]
	if last {
		b.Cursor = keys[len(keys)-1]
	}
	return b
}

func (b Browser) fold() Browser {
	sid, _, isRow := strings.Cut(b.Cursor, "/")
	if !isRow {
		b.Folds[sid] = !b.folded(sid)
		return b
	}
	for _, l := range b.lines(token.SplitWidth) {
		if l.Key == b.Cursor && l.Head {
			b.Folds[b.Cursor] = !b.Folds[b.Cursor]
			return b
		}
	}
	b.Folds[sid] = true
	b.Cursor = sid
	return b
}

func (b Browser) folded(key string) bool {
	if b.filter.Value() != "" {
		return false
	}
	return b.Folds[key]
}

func (b Browser) keys() []string {
	var keys []string
	for _, l := range b.lines(token.SplitWidth) {
		if l.Key != "" && (l.Kind == organism.LineTitle || l.Kind == organism.LineRow) && !slices.Contains(keys, l.Key) {
			keys = append(keys, l.Key)
		}
	}
	return keys
}

func (b Browser) match(r page.Row) bool {
	needle := strings.ToLower(b.filter.Value())
	if needle == "" {
		return true
	}
	if strings.Contains(strings.ToLower(r.Label), needle) || strings.Contains(strings.ToLower(r.ID), needle) {
		return true
	}
	for _, c := range r.Cells {
		if strings.Contains(strings.ToLower(atom.Plain(c)), needle) {
			return true
		}
	}
	return false
}

func (b Browser) lookup(key string) (string, bool) {
	for _, a := range b.Page.Actions {
		if a.Key == key {
			return a.Label, true
		}
	}
	return "", false
}

func (b Browser) lines(width int) []organism.Line {
	st := b.styles
	var out []organism.Line
	for _, s := range b.Page.Sections {
		g := organism.Grid{Styles: st, Section: s, Width: width, Medium: molecule.TUI, Keys: b.lookup, Folded: b.folded, Match: b.match}
		body := g.Lines()
		if b.filter.Value() != "" && !slices.ContainsFunc(body, func(l organism.Line) bool { return l.Kind == organism.LineRow && l.Key != "" }) {
			continue
		}
		out = append(out, organism.Line{Key: s.ID, Kind: organism.LineTitle, Text: organism.FoldTitle(st, s, b.folded(s.ID), width)})
		if b.folded(s.ID) {
			continue
		}
		out = append(out, body...)
	}
	return out
}

func (b Browser) Body(width, height int) string {
	if b.Page.ID == "" {
		return ""
	}
	st := b.styles
	var top []string
	if b.Page.Strip != nil {
		top = append(top, organism.Strip(st, *b.Page.Strip, width))
	}
	if b.focus == Filter || b.filter.Value() != "" {
		top = append(top, atom.Spaces(token.Margin)+b.filter.View())
	}
	room := max(height-len(top), 1)
	if b.Page.Error != nil {
		body := make([]string, 0, 2)
		for _, l := range molecule.ErrorLines(st, *b.Page.Error) {
			body = append(body, atom.Spaces(token.Margin)+l)
		}
		return strings.Join(append(top, body...), "\n")
	}
	if b.focus == Sheet {
		return strings.Join(organism.Sheet(st, b.Tab, b.Bindings(), width, height), "\n")
	}
	_, row, isRow := b.Current()
	section, _, _ := b.Current()
	listWidth := width
	split := b.detail && isRow && width >= token.SplitWidth
	if split {
		listWidth = width - token.PaneWidth - 1
	}
	var list []string
	if b.detail && isRow && !split {
		list = organism.Detail(st, section, row, b.Bindings(), width)
	} else {
		list = b.list(listWidth, room)
	}
	if split {
		pane := organism.Detail(st, section, row, b.Bindings(), token.PaneWidth)
		for i := range list {
			l := atom.Pad(list[i], listWidth)
			if i < len(pane) {
				l += st.Subtle.Render(token.Divider) + pane[i]
			}
			list[i] = l
		}
	}
	var modal []string
	switch {
	case b.ask != nil:
		modal = strings.Split(organism.Modal(st, b.ask.confirm, b.ask.yes, width), "\n")
	case b.form != nil:
		modal = strings.Split(organism.Frame(st, "", strings.Split(strings.TrimRight(b.form.f.View(), "\n"), "\n"), min(width-4, token.ModalWidth)), "\n")
	}
	if modal != nil {
		keep := max(room-len(modal), 0)
		if len(list) > keep {
			list = list[:keep]
		}
		for len(list) < keep {
			list = append(list, "")
		}
		list = append(list, modal...)
	}
	return strings.Join(append(top, list...), "\n")
}

func (b Browser) list(width, room int) []string {
	st := b.styles
	lines := b.lines(width)
	if len(lines) == 0 {
		empty := atom.Spaces(token.Margin+token.Indent) + molecule.Empty(st, page.Empty{Noun: b.Noun}, molecule.TUI, b.lookup)
		if b.filter.Value() != "" {
			empty += atom.Spaces(token.Gap) + st.Muted.Render(token.Arrow+" / to change the filter")
		}
		return []string{empty}
	}
	at := slices.IndexFunc(lines, func(l organism.Line) bool { return l.Key == b.Cursor })
	start := 0
	sticky := ""
	if at >= room {
		start = at - room + 2
		for i := start; i >= 0; i-- {
			if lines[i].Kind == organism.LineTitle {
				sticky = lines[i].Text
				break
			}
		}
	}
	out := make([]string, 0, room)
	if sticky != "" {
		out = append(out, sticky)
	}
	for i := start; i < len(lines) && len(out) < room; i++ {
		l := lines[i]
		text := l.Text
		if l.Key == b.Cursor && (l.Kind == organism.LineRow || l.Kind == organism.LineTitle) {
			text = st.Selection.Width(max(width-1, 1)).Render(text)
		}
		out = append(out, text)
	}
	return out
}

func (b Browser) Bindings() []organism.Binding {
	var out []organism.Binding
	actions := slices.Clone(b.Page.Actions)
	slices.SortStableFunc(actions, func(x, y page.Action) int { return x.Priority - y.Priority })
	for _, a := range actions {
		out = append(out, organism.Binding{Key: a.Key, Label: a.Label, Group: organism.GroupTab, Mutates: a.Mutates})
	}
	return append(out, keyMap()...)
}

func keyMap() []organism.Binding {
	list := func(key, label string) organism.Binding {
		return organism.Binding{Key: key, Label: label, Group: organism.GroupList}
	}
	global := func(key, label string) organism.Binding {
		return organism.Binding{Key: key, Label: label, Group: organism.GroupGlobal}
	}
	return []organism.Binding{
		list("up/down", "move"), list("enter", "detail"), list("/", "filter"), list("space", "fold"),
		list("g G", "top, bottom"), list("esc", "back, clear"),
		global("r", "reload"), global("q", "quit"), global("1-6", "tab"), global("tab", "next tab"),
	}
}

func (b Browser) Footer(width int, busy bool) string {
	st := b.styles
	if b.focus == Filter {
		return atom.Spaces(token.Margin) + b.filter.View()
	}
	var keys []organism.Binding
	if b.Page.Error != nil {
		keys = append(keys, organism.Binding{Key: "r", Label: "retry"})
	}
	if b.focus == Detail {
		keys = append(keys, organism.Binding{Key: "esc", Label: "back"})
	}
	keys = append(keys, organism.Binding{Key: "up/down", Label: "move"})
	_, row, isRow := b.Current()
	for _, x := range b.Bindings() {
		switch {
		case x.Group == organism.GroupTab:
			if a, _, ok := b.action(x.Key); ok && (a.On == page.OnPage || (isRow && (len(a.Rows) == 0 || slices.Contains(a.Rows, row.ID)))) {
				keys = append(keys, x)
			}
		case x.Key == "r" && b.Page.Error != nil:
		case x.Key == "enter", x.Key == "/", x.Key == "space", x.Key == "r", x.Key == "q":
			keys = append(keys, x)
		}
	}
	return organism.Footer(st, keys, busy, width)
}

func (b Browser) Status(m page.Mark, text, hint string, width int) string {
	return organism.StatusBar(b.styles, m, text, hint, width)
}
