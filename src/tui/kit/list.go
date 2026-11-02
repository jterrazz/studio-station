package kit

import (
	"context"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/jterrazz/studio-station/src/output"
	"github.com/jterrazz/studio-station/src/present"
	"github.com/jterrazz/studio-station/src/theme"
)

type Row struct {
	Key  string
	Row  present.Row
	Data any
}

type Group struct {
	Title  string
	Counts []present.Count
	Rows   []Row
}

type Loaded struct {
	Screen string
	Groups []Group
	Note   Status
	Err    error
	gen    int
	more   <-chan []Group
	final  bool
}

type ReadDone struct {
	Screen string
	Title  string
	Doc    present.Document
	Err    error
}

type ListConfig struct {
	ID   string
	Load func() ([]Group, Status, error)
	// value: a screen whose reading fills in steps watches it instead: the
	// frame at once, then each value; fresh drops the run's shared reading. A
	// reading that cannot start answers its error, shown as a failed read.
	Watch     func(ctx context.Context, fresh bool) (<-chan []Group, error)
	Describe  Describe
	Empty     string
	EmptyHint string
	FoldAllOK bool
	Detail    func(Row) (title string, doc present.Document, ok bool)
	Keys      []key.Binding
}

type pane struct {
	title string
	doc   present.Document
}

type ListScreen struct {
	cfg       ListConfig
	theme     theme.Theme
	groups    []Group
	folded    map[string]bool
	cursor    int
	loaded    bool
	loading   bool
	failure   Status
	filter    textinput.Model
	filtering bool
	detail    *pane
	busy      bool
	rowBusy   string
	modal     *Modal
	spinner   spinner.Model
	gen       int
	// invariant: Init runs on a copy, so the running watch's cancel is held
	// where every copy of the screen sees it.
	stop *context.CancelFunc
}

type item struct {
	group, row int
}

func NewList(cfg ListConfig, t theme.Theme) ListScreen {
	filter := textinput.New()
	filter.Prompt = "/ "
	filter.Placeholder = "filter"
	return ListScreen{
		cfg:     cfg,
		theme:   t,
		folded:  map[string]bool{},
		loading: true,
		stop:    new(context.CancelFunc),
		filter:  filter,
		spinner: spinner.New(spinner.WithSpinner(spinner.Dot), spinner.WithStyle(t.Accent)),
	}
}

func (l ListScreen) Init() tea.Cmd {
	return tea.Batch(l.spinner.Tick, l.watch(false))
}

func (l ListScreen) start(fresh bool) (ListScreen, tea.Cmd) {
	l.gen++
	return l, l.watch(fresh)
}

func (l ListScreen) watch(fresh bool) tea.Cmd {
	id, gen := l.cfg.ID, l.gen
	if l.cfg.Watch == nil {
		load := l.cfg.Load
		return func() tea.Msg {
			groups, note, err := load()
			return Loaded{Screen: id, Groups: groups, Note: note, Err: err, gen: gen, final: true}
		}
	}
	if *l.stop != nil {
		(*l.stop)()
	}
	ctx, stop := context.WithCancel(context.Background()) //nolint:gosec // outside: held in *l.stop, called when the next watch starts.
	*l.stop = stop
	watch := l.cfg.Watch
	return func() tea.Msg {
		values, err := watch(ctx, fresh)
		if err != nil {
			return Loaded{Screen: id, Err: err, gen: gen, final: true}
		}
		return next(id, gen, values)()
	}
}

func Groups[T any](ctx context.Context, src <-chan T, groups func(T) []Group) <-chan []Group {
	out := make(chan []Group)
	go func() {
		defer close(out)
		for v := range src {
			select {
			case out <- groups(v):
			case <-ctx.Done():
				return
			}
		}
	}()
	return out
}

func next(id string, gen int, ch <-chan []Group) tea.Cmd {
	return func() tea.Msg {
		groups, open := <-ch
		return Loaded{Screen: id, Groups: groups, gen: gen, more: ch, final: !open}
	}
}

func (l ListScreen) Focus() Focus {
	switch {
	case l.modal != nil:
		return FocusModal
	case l.filtering:
		return FocusFilter
	}
	return FocusNormal
}

func (l ListScreen) Busy() bool { return l.busy }

func (l ListScreen) Loading() bool { return l.loading }

func (l ListScreen) Loaded() bool { return l.loaded && l.failure.Text == "" }

func (l ListScreen) Filter() string { return l.filter.Value() }

func (l ListScreen) Keys() []key.Binding {
	return append([]key.Binding{MoveKey(), FoldKey()}, l.cfg.Keys...)
}

func (l ListScreen) Current() (Row, bool) {
	items := l.items()
	if l.cursor < 0 || l.cursor >= len(items) || items[l.cursor].row < 0 {
		return Row{}, false
	}
	it := items[l.cursor]
	return l.groups[it.group].Rows[it.row], true
}

func (l ListScreen) Rows() []Row {
	var rows []Row
	for _, g := range l.groups {
		rows = append(rows, g.Rows...)
	}
	return rows
}

func (l ListScreen) hasRows() bool {
	for _, r := range l.Rows() {
		if r.Key != "" {
			return true
		}
	}
	return false
}

func (l ListScreen) Act(cmd tea.Cmd) (ListScreen, tea.Cmd) {
	if cmd == nil {
		return l, nil
	}
	l.busy = true
	return l, cmd
}

func (l ListScreen) Open(m *Modal) (ListScreen, tea.Cmd) {
	l.modal = m
	return l, m.init()
}

func (l ListScreen) Read(rowKey string, cmd tea.Cmd) (ListScreen, tea.Cmd) {
	l.rowBusy = rowKey
	return l, tea.Batch(l.spinner.Tick, cmd)
}

func (l ListScreen) Update(msg tea.Msg) (ListScreen, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.BackgroundColorMsg:
		l.theme = theme.New(msg.IsDark())
		l.spinner.Style = l.theme.Accent
	case Loaded:
		if msg.Screen == l.cfg.ID {
			return l.loadedWith(msg)
		}
		return l, nil
	case Done:
		if msg.Screen != l.cfg.ID {
			return l, nil
		}
		l.busy = false
		status := msg.Status
		if msg.Err != nil {
			text, hint := l.cfg.Describe(msg.Err)
			status = Status{Mark: present.MarkBroken, Text: text, Hint: hint}
		}
		l.loading = true
		started, cmd := l.start(true)
		return started, tea.Batch(Say(status), cmd)
	case ReadDone:
		if msg.Screen != l.cfg.ID {
			return l, nil
		}
		l.rowBusy = ""
		if msg.Err != nil {
			text, hint := l.cfg.Describe(msg.Err)
			return l, Say(Status{Mark: present.MarkBroken, Text: text, Hint: hint})
		}
		l.detail = &pane{title: msg.Title, doc: msg.Doc}
		return l, nil
	case Reload:
		if l.busy {
			return l, nil
		}
		l.loading = true
		started, cmd := l.start(true)
		return started, tea.Batch(l.spinner.Tick, cmd)
	case spinner.TickMsg:
		if !l.loaded || l.rowBusy != "" {
			var cmd tea.Cmd
			l.spinner, cmd = l.spinner.Update(msg)
			return l, cmd
		}
		return l, nil
	}
	if l.modal != nil {
		return l.updateModal(msg)
	}
	k, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return l, nil
	}
	if l.filtering {
		return l.updateFilter(k)
	}
	return l.navigate(k), nil
}

func (l ListScreen) loadedWith(msg Loaded) (ListScreen, tea.Cmd) {
	if msg.gen != l.gen {
		return l, nil
	}
	if msg.more != nil {
		if msg.final {
			l.loading = false
			return l, nil
		}
		l = l.show(msg.Groups)
		return l, next(l.cfg.ID, msg.gen, msg.more)
	}
	l.loading = false
	if msg.Err != nil {
		text, hint := l.cfg.Describe(msg.Err)
		l.failure = Status{Mark: present.MarkBroken, Text: text, Hint: hint}
		return l, nil
	}
	l = l.show(msg.Groups)
	if msg.Note.Text != "" {
		return l, Say(msg.Note)
	}
	return l, nil
}

// invariant: a group's fold is decided once its rows have answered, so a
// frame of pending rows never folds a group the full reading leaves open.
func (l ListScreen) show(groups []Group) ListScreen {
	l.failure = Status{}
	for _, g := range groups {
		if _, seen := l.folded[g.Title]; !seen && g.Title != "" && !pendingIn(g) {
			l.folded[g.Title] = l.cfg.FoldAllOK && allOK(g)
		}
	}
	l.groups = groups
	l.loaded = true
	l.cursor = min(l.cursor, max(len(l.items())-1, 0))
	return l
}

func pendingIn(g Group) bool {
	for _, r := range g.Rows {
		if r.Row.Mark == present.MarkPending {
			return true
		}
	}
	return false
}

func allOK(g Group) bool {
	for _, r := range g.Rows {
		if r.Row.Mark != present.MarkOK {
			return false
		}
	}
	return len(g.Rows) > 0
}

func (l ListScreen) updateModal(msg tea.Msg) (ListScreen, tea.Cmd) {
	if k, ok := msg.(tea.KeyPressMsg); ok && (k.String() == "esc" || k.String() == "ctrl+c") {
		l.modal = nil
		return l, nil
	}
	done, cmd := l.modal.update(msg)
	if !done {
		return l, cmd
	}
	submit := l.modal.result()
	l.modal = nil
	if submit == nil {
		return l, cmd
	}
	l.busy = true
	return l, tea.Batch(cmd, submit)
}

func (l ListScreen) updateFilter(k tea.KeyPressMsg) (ListScreen, tea.Cmd) {
	switch k.String() {
	case "esc":
		l.filter.Reset()
		l.filter.Blur()
		l.filtering = false
	case "enter":
		l.filter.Blur()
		l.filtering = false
	default:
		var cmd tea.Cmd
		l.filter, cmd = l.filter.Update(k)
		l.cursor = 0
		return l, cmd
	}
	l.cursor = min(l.cursor, max(len(l.items())-1, 0))
	return l, nil
}

func (l ListScreen) navigate(k tea.KeyPressMsg) ListScreen {
	items := l.items()
	switch k.String() {
	case "up", "k":
		l.cursor = max(l.cursor-1, 0)
	case "down", "j":
		l.cursor = min(l.cursor+1, max(len(items)-1, 0))
	case "g", "home":
		l.cursor = 0
	case "G", "end":
		l.cursor = max(len(items)-1, 0)
	case "space":
		if l.cursor < len(items) {
			g := l.groups[items[l.cursor].group]
			if g.Title != "" {
				l.folded[g.Title] = !l.folded[g.Title]
				l = l.onHeader(items[l.cursor].group)
			}
		}
	case "/":
		l.filtering = true
		l.filter.Focus()
	case "enter":
		if l.detail != nil {
			l.detail = nil
			break
		}
		if row, ok := l.Current(); ok && l.cfg.Detail != nil {
			if title, doc, has := l.cfg.Detail(row); has {
				l.detail = &pane{title: title, doc: doc}
			}
		}
	case "esc":
		l.detail = nil
	}
	return l
}

func (l ListScreen) CloseDetail() ListScreen {
	l.detail = nil
	return l
}

func (l ListScreen) onHeader(group int) ListScreen {
	for i, it := range l.items() {
		if it.group == group && it.row < 0 {
			l.cursor = i
		}
	}
	return l
}

func (l ListScreen) items() []item {
	needle := strings.ToLower(l.filter.Value())
	var out []item
	for gi, g := range l.groups {
		var rows []item
		for ri, r := range g.Rows {
			if needle == "" || matches(r, needle) {
				rows = append(rows, item{group: gi, row: ri})
			}
		}
		if needle != "" && len(rows) == 0 {
			continue
		}
		if g.Title != "" {
			out = append(out, item{group: gi, row: -1})
			if needle == "" && l.folded[g.Title] {
				continue
			}
		}
		out = append(out, rows...)
	}
	return out
}

func matches(r Row, needle string) bool {
	if strings.Contains(strings.ToLower(r.Key), needle) {
		return true
	}
	for _, c := range r.Row.Cells {
		if strings.Contains(strings.ToLower(c.Text), needle) {
			return true
		}
	}
	return false
}

func (l ListScreen) View(width, height int) string {
	t := l.theme
	var top []string
	if l.filtering || l.filter.Value() != "" {
		top = append(top, margin+l.filter.View())
	}
	var bottom string
	switch {
	case l.modal != nil:
		bottom = l.modal.view(t, width)
	case l.detail != nil:
		bottom = Frame(t, l.detail.title, output.TTYBody(l.detail.doc, width-6, t), width-1)
	}
	room := height - len(top) - lipgloss.Height(bottom)
	if bottom == "" {
		room = height - len(top)
	}
	var body []string
	switch {
	case l.failure.Text != "":
		body = []string{
			margin + MarkText(t, present.MarkBroken) + " " + t.Bad.Render(l.failure.Text),
			margin + "  " + t.Muted.Render(l.failure.Hint),
			margin + "  " + t.Muted.Render("press r to retry"),
		}
	case !l.loaded:
		body = []string{margin + l.spinner.View() + " " + t.Muted.Render("loading")}
	case !l.hasRows():
		body = []string{rowIndent + t.Muted.Render(l.cfg.Empty), rowIndent + t.Muted.Render(l.cfg.EmptyHint)}
	default:
		body = l.lines(width, max(room, 1))
	}
	out := make([]string, 0, len(top)+len(body)+height)
	out = append(out, top...)
	out = append(out, body...)
	for len(out) < height-lipgloss.Height(bottom) && bottom != "" {
		out = append(out, "")
	}
	view := strings.Join(out, "\n")
	if bottom != "" {
		view += "\n" + bottom
	}
	return view
}

func (l ListScreen) lines(width, room int) []string {
	items := l.items()
	widths := l.widths()
	start := 0
	if l.cursor >= room {
		start = l.cursor - room + 1
	}
	end := min(start+room, len(items))
	out := make([]string, 0, end-start)
	for i := start; i < end; i++ {
		it := items[i]
		g := l.groups[it.group]
		if it.row < 0 {
			out = append(out, l.header(g, width, i == l.cursor))
			continue
		}
		out = append(out, l.row(g.Rows[it.row], widths, width, i == l.cursor))
	}
	return out
}

func (l ListScreen) header(g Group, width int, selected bool) string {
	t := l.theme
	sign := "-"
	if l.folded[g.Title] && l.filter.Value() == "" {
		sign = "+"
	}
	left := margin + t.Muted.Render(sign) + " " + t.Title.Render(g.Title)
	right := CountsText(t, g.Counts)
	gap := max(width-1-lipgloss.Width(left)-lipgloss.Width(right), 2)
	line := left + strings.Repeat(" ", gap) + right
	if selected {
		return lipgloss.NewStyle().Background(t.Tokens.Selection).Width(max(width-1, 1)).Render(line)
	}
	return line
}

func (l ListScreen) row(r Row, widths []int, width int, selected bool) string {
	t := l.theme
	var line strings.Builder
	line.WriteString(rowIndent)
	switch {
	case r.Key != "" && r.Key == l.rowBusy:
		line.WriteString(l.spinner.View() + " ")
	case r.Row.Mark != present.NoMark:
		line.WriteString(MarkText(t, r.Row.Mark) + " ")
	default:
		line.WriteString(cellGap)
	}
	used := len(rowIndent) + 2
	for i, c := range r.Row.Cells {
		if i == len(r.Row.Cells)-1 {
			text := Truncate(c.Text, max(width-1-used, 1))
			line.WriteString(Cell(t, present.Cell{Text: text, Tone: c.Tone}))
			break
		}
		line.WriteString(Cell(t, c) + strings.Repeat(" ", max(widths[i]-lipgloss.Width(c.Text), 0)) + cellGap)
		used += widths[i] + len(cellGap)
	}
	text := strings.TrimRight(line.String(), " ")
	if lipgloss.Width(text) > width-1 {
		// invariant: a row is one line; columns wider than the screen are cut
		// here, or the selection's fixed width would wrap the row onto a second one.
		text = lipgloss.NewStyle().MaxWidth(max(width-2, 1)).Render(text) + "…"
	}
	if selected {
		return lipgloss.NewStyle().Background(t.Tokens.Selection).Width(max(width-1, 1)).Render(text)
	}
	return text
}

func (l ListScreen) widths() []int {
	var widths []int
	for _, g := range l.groups {
		for _, r := range g.Rows {
			for i, c := range r.Row.Cells {
				if i >= len(widths) {
					widths = append(widths, 0)
				}
				widths[i] = max(widths[i], lipgloss.Width(c.Text))
			}
		}
	}
	return widths
}

func GroupsOf(doc present.Document) []Group {
	var groups []Group
	loose := Group{}
	for _, b := range doc.Blocks {
		switch v := b.(type) {
		case present.Section:
			g := Group{Title: v.Title, Counts: v.Counts}
			for _, inner := range v.Blocks {
				if table, ok := inner.(present.Table); ok {
					g.Rows = append(g.Rows, rowsOf(table)...)
				}
			}
			groups = append(groups, g)
		case present.Table:
			loose.Rows = append(loose.Rows, rowsOf(v)...)
		}
	}
	if len(loose.Rows) > 0 {
		groups = append([]Group{loose}, groups...)
	}
	return groups
}

func rowsOf(t present.Table) []Row {
	rows := make([]Row, 0, len(t.Rows))
	for _, r := range t.Rows {
		k := ""
		if len(r.Cells) > 0 && r.Mark != present.NoMark {
			k = r.Cells[0].Text
		}
		rows = append(rows, Row{Key: k, Row: r})
	}
	return rows
}
