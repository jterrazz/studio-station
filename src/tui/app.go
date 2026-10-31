package tui

import (
	"strconv"
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/jterrazz/studio-station/src/theme"
	"github.com/jterrazz/studio-station/src/tui/kit"
)

type App struct {
	tabs    []kit.Tab
	screens []kit.Screen
	active  int
	context string
	width   int
	height  int
	theme   theme.Theme
	help    help.Model
	status  kit.Status
}

func newApp(context string, tabs []kit.Tab, screens []kit.Screen) App {
	t := theme.New(true)
	return App{tabs: tabs, screens: screens, context: context, width: 80, height: 24, theme: t, help: kit.NewHelp(t)}
}

func (a App) Init() tea.Cmd {
	cmds := make([]tea.Cmd, 0, 1+len(a.screens))
	cmds = append(cmds, tea.RequestBackgroundColor)
	for _, s := range a.screens {
		cmds = append(cmds, s.Init())
	}
	return tea.Batch(cmds...)
}

func globalKeys() []key.Binding {
	return []key.Binding{
		kit.Bind("tab", "next tab", "tab"),
		kit.Bind("1-5", "tab", "1", "2", "3", "4", "5"),
		kit.Bind("r", "reload", "r"),
		kit.Bind("?", "help", "?"),
		kit.Bind("q", "quit", "q", "ctrl+c"),
	}
}

func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		if msg.Width > 0 && msg.Height > 0 {
			a.width, a.height = msg.Width, msg.Height
		}
	case tea.BackgroundColorMsg:
		a.theme = theme.New(msg.IsDark())
		a.help = kit.NewHelp(a.theme)
	case kit.Status:
		a.status = msg
		return a, nil
	case kit.ExecMsg:
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
	a.status = kit.Status{}
	current := a.screens[a.active]
	if current.Focus() != kit.FocusNormal {
		return a.forward(k)
	}
	switch s := k.String(); s {
	case "q", "ctrl+c":
		return a, tea.Quit
	case "tab":
		return a.show((a.active + 1) % len(a.screens))
	case "shift+tab":
		return a.show((a.active + len(a.screens) - 1) % len(a.screens))
	case "r":
		return a.forward(kit.Reload{})
	case "?":
		a.help.ShowAll = !a.help.ShowAll
		return a, nil
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
	a.screens[a.active], hide = a.screens[a.active].Update(kit.Shown{Visible: false})
	a.active = n
	a.screens[a.active], show = a.screens[a.active].Update(kit.Shown{Visible: true})
	return a, tea.Batch(hide, show)
}

func (a App) View() tea.View {
	t := a.theme
	current := a.screens[a.active]
	header := kit.Header(t, a.tabs, a.active, a.context, a.width)
	footer := kit.Footer(t, a.help, current.Keys(), globalKeys(), current.Busy(), a.width)
	bar := kit.StatusLine(t, a.status, a.width)
	room := max(a.height-lipgloss.Height(header)-lipgloss.Height(footer)-1, 1)
	body := current.View(a.width, room)
	lines := strings.Split(body, "\n")
	if len(lines) > room {
		lines = lines[:room]
	}
	for len(lines) < room {
		lines = append(lines, "")
	}
	v := tea.NewView(header + "\n" + strings.Join(lines, "\n") + "\n" + bar + "\n" + footer)
	v.AltScreen = true
	v.WindowTitle = "station"
	return v
}
