package tui

import (
	"strconv"

	tea "charm.land/bubbletea/v2"

	"github.com/jterrazz/studio-station/src/page"
	"github.com/jterrazz/studio-station/src/tui/screens/tab"
	ui "github.com/jterrazz/studio-station/src/ui/render/tui"
)

type App struct {
	tabs    []Tab
	screens []tab.Screen
	active  int
	here    page.Context
	width   int
	height  int
	chrome  ui.Chrome
	status  tab.Status
	prefer  func(prefix string)
	stop    func()
	turning bool
	frame   *frame
}

// invariant: the frame is drawn again only when a message can change it; an
// answer for a hidden tab, or a spinner tick with nothing loading, reuses the
// last one, so a burst of hidden reads never queues a key behind redraws.
type frame struct {
	view  tea.View
	drawn bool
}

func newApp(here page.Context, tabs []Tab, screens []tab.Screen, prefer func(string)) App {
	if prefer == nil {
		prefer = func(string) {}
	}
	for i := range screens {
		if i > 0 {
			screens[i], _ = screens[i].Update(tab.Shown{Visible: false})
		}
	}
	return App{tabs: tabs, screens: screens, here: here, width: 80, height: 24, chrome: ui.NewChrome(), prefer: prefer, turning: true, frame: &frame{}}
}

func (a App) Init() tea.Cmd {
	a.prefer(a.tabs[a.active].Probes)
	cmds := make([]tea.Cmd, 0, 2+len(a.screens))
	cmds = append(cmds, tea.RequestBackgroundColor, a.chrome.Tick)
	for _, s := range a.screens {
		cmds = append(cmds, s.Init())
	}
	return tea.Batch(cmds...)
}

func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if !a.screens[a.active].Owns(msg) {
		for i, s := range a.screens {
			if s.Owns(msg) {
				var cmd tea.Cmd
				a.screens[i], cmd = s.Update(msg)
				return a, cmd
			}
		}
		return a, nil
	}
	if chrome, cmd, ok := a.chrome.Turn(msg); ok {
		// invariant: the header spinner turns while the shown tab has a probe
		// pending, and stops, costing no redraw, once it has answered.
		if !a.screens[a.active].Loading() {
			a.turning = false
			return a, nil
		}
		a.chrome, a.frame.drawn = chrome, false
		return a, cmd
	}
	a.frame.drawn = false
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		if msg.Width <= 0 || msg.Height <= 0 {
			// hazard: a terminal that reports no size (a pty `script` opens over a
			// pipe) leaves the renderer 0x0, and it draws nothing; resizing it to the
			// model's own size makes the first frame appear.
			size := tea.WindowSizeMsg{Width: a.width, Height: a.height}
			return a, func() tea.Msg { return size }
		}
		a.width, a.height = msg.Width, msg.Height
	case tea.BackgroundColorMsg:
		a.chrome = a.chrome.Themed(msg.IsDark())
	case tab.Status:
		a.status = msg
		return a, nil
	case tab.ExecMsg:
		return a, tea.Exec(msg.Command, msg.Done)
	case tea.KeyPressMsg:
		return a.key(msg)
	}
	cmds := make([]tea.Cmd, 0, len(a.screens))
	for i, s := range a.screens {
		var cmd tea.Cmd
		a.screens[i], cmd = s.Update(msg)
		cmds = append(cmds, cmd)
	}
	return a, tea.Batch(cmds...)
}

func (a App) key(k tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	a.status = tab.Status{}
	if a.screens[a.active].Modal() {
		return a.forward(k)
	}
	switch s := k.String(); s {
	case "q", "ctrl+c":
		// invariant: q stops every read at once (the pool's walks and commands
		// are cancelled), so the program's own shutdown is not left to share the
		// machine with them; the pool's Close after the program reaps them.
		if a.stop != nil {
			go a.stop()
		}
		return a, tea.Quit
	case "tab":
		return a.show((a.active + 1) % len(a.screens))
	case "shift+tab":
		return a.show((a.active + len(a.screens) - 1) % len(a.screens))
	case "r":
		model, cmd := a.forward(tab.Reload{})
		app, _ := model.(App)
		return app.turn(cmd)
	default:
		if n, err := strconv.Atoi(s); err == nil && n >= 1 && n <= len(a.screens) {
			return a.show(n - 1)
		}
	}
	return a.forward(k)
}

func (a App) forward(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	a.screens[a.active], cmd = a.screens[a.active].Update(msg)
	return a, cmd
}

func (a App) show(n int) (tea.Model, tea.Cmd) {
	if n == a.active {
		return a, nil
	}
	var hide, show tea.Cmd
	a.screens[a.active], hide = a.screens[a.active].Update(tab.Shown{Visible: false})
	a.active = n
	a.prefer(a.tabs[n].Probes)
	a.screens[a.active], show = a.screens[a.active].Update(tab.Shown{Visible: true})
	return a.turn(tea.Batch(hide, show))
}

func (a App) turn(cmd tea.Cmd) (tea.Model, tea.Cmd) {
	if a.turning || !a.screens[a.active].Loading() {
		return a, cmd
	}
	a.turning = true
	return a, tea.Batch(cmd, a.chrome.Tick)
}

func (a App) View() tea.View {
	if a.frame.drawn {
		return a.frame.view
	}
	current := a.screens[a.active]
	labels := make([]ui.Tab, 0, len(a.tabs))
	for _, t := range a.tabs {
		labels = append(labels, t.Tab)
	}
	header := a.chrome.Header(labels, a.active, a.here, current.Loading(), a.width)
	body := func(room int) string { return current.Body(a.width, room) }
	v := tea.NewView(a.chrome.Screen(header, body, a.chrome.StatusBar(a.status.Mark, a.status.Text, a.status.Hint, a.width), current.Footer(a.width), a.height))
	v.AltScreen = true
	v.WindowTitle = "station"
	a.frame.view, a.frame.drawn = v, true
	return v
}
