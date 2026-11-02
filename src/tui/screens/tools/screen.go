package toolsscreen

import (
	"context"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/domain/tools"
	"github.com/jterrazz/studio-station/src/present"
	presenttools "github.com/jterrazz/studio-station/src/present/tools"
	"github.com/jterrazz/studio-station/src/theme"
	"github.com/jterrazz/studio-station/src/tui/kit"
)

const ID = "tools"

type Tools interface {
	Watch(ctx context.Context) <-chan []tools.Report
	Forget()
	CheckInstall(ctx context.Context, names []string) error
	Install(ctx context.Context, r tools.InstallRequest) ([]tools.Outcome, error)
	CheckUninstall(names []string) error
	Uninstall(ctx context.Context, names []string) ([]tools.Outcome, error)
	CheckUpgrade(names []string) error
	Upgrade(ctx context.Context, r tools.UpgradeRequest) ([]tools.Outcome, error)
}

type Model struct {
	ctx   context.Context
	tools Tools
	list  kit.ListScreen
}

func New(ctx context.Context, t Tools, describe kit.Describe, th theme.Theme) Model {
	m := Model{ctx: ctx, tools: t}
	m.list = kit.NewList(kit.ListConfig{
		ID:        ID,
		Watch:     m.watch,
		Describe:  describe,
		Empty:     "none",
		EmptyHint: "press r to reload the catalogue",
		FoldAllOK: true,
		Detail: func(r kit.Row) (string, present.Document, bool) {
			report, ok := r.Data.(tools.Report)
			return r.Key, presenttools.Detail(report), ok
		},
		Keys: []key.Binding{
			kit.Bind("i", "install", "i"),
			kit.Bind("u", "uninstall", "u"),
			kit.FilterKey(),
			kit.Bind("U", "upgrade", "U"),
			kit.DetailKey(),
		},
	}, th)
	return m
}

func (m Model) watch(ctx context.Context, fresh bool) (<-chan []kit.Group, error) {
	if fresh {
		m.tools.Forget()
	}
	return kit.Groups(ctx, m.tools.Watch(ctx), groups), nil
}

func groups(reports []tools.Report) []kit.Group {
	byName := make(map[string]tools.Report, len(reports))
	for _, r := range reports {
		byName[r.Tool.Name] = r
	}
	groups := kit.GroupsOf(presenttools.Catalogue(reports, "").Doc)
	for gi := range groups {
		for ri := range groups[gi].Rows {
			if r, ok := byName[groups[gi].Rows[ri].Key]; ok {
				groups[gi].Rows[ri].Data = r
			}
		}
	}
	return groups
}

func (m Model) Init() tea.Cmd { return m.list.Init() }

func (m Model) Focus() kit.Focus { return m.list.Focus() }

func (m Model) Busy() bool { return m.list.Busy() }

func (m Model) Loading() bool { return m.list.Loading() }

func (m Model) Keys() []key.Binding { return m.list.Keys() }

func (m Model) View(width, height int) string { return m.list.View(width, height) }

func (m Model) Update(msg tea.Msg) (kit.Screen, tea.Cmd) {
	if k, ok := msg.(tea.KeyPressMsg); ok && m.list.Focus() == kit.FocusNormal {
		if cmd, handled := m.action(k); handled {
			return m, cmd
		}
	}
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m *Model) action(k tea.KeyPressMsg) (tea.Cmd, bool) {
	switch k.String() {
	case "i", "u", "U":
	default:
		return nil, false
	}
	row, ok := m.list.Current()
	if !ok || m.list.Busy() {
		return nil, true
	}
	name := row.Key
	var cmd tea.Cmd
	switch k.String() {
	case "i":
		m.list, cmd = m.list.Open(kit.Confirm("Install "+name+"?", m.install(name)))
	case "u":
		m.list, cmd = m.list.Open(kit.Confirm("Uninstall "+name+"?", m.uninstall(name)))
	case "U":
		m.list, cmd = m.list.Act(m.upgrade(name))
	}
	return cmd, true
}

func (m Model) install(name string) tea.Cmd {
	return kit.Exec(ID, func() (kit.Status, error) {
		if err := m.tools.CheckInstall(m.ctx, []string{name}); err != nil {
			return kit.Status{}, err
		}
		outcomes, err := m.tools.Install(m.ctx, tools.InstallRequest{Names: []string{name}, Interactive: true})
		return outcome("installed", name, outcomes), err
	})
}

func (m Model) uninstall(name string) tea.Cmd {
	return kit.Exec(ID, func() (kit.Status, error) {
		if err := m.tools.CheckUninstall([]string{name}); err != nil {
			return kit.Status{}, err
		}
		outcomes, err := m.tools.Uninstall(m.ctx, []string{name})
		return outcome("uninstalled", name, outcomes), err
	})
}

func (m Model) upgrade(name string) tea.Cmd {
	return kit.Exec(ID, func() (kit.Status, error) {
		if err := m.tools.CheckUpgrade([]string{name}); err != nil {
			return kit.Status{}, err
		}
		outcomes, err := m.tools.Upgrade(m.ctx, tools.UpgradeRequest{Names: []string{name}})
		return outcome("upgraded", name, outcomes), err
	})
}

// value: the status bar names the tool asked for; a Then step that failed
// degrades the line rather than hiding behind the tool's own success.
func outcome(done, name string, outcomes []tools.Outcome) kit.Status {
	for _, o := range outcomes {
		if o.Name != name {
			continue
		}
		state := o.Result
		for _, s := range o.Steps {
			if s.Result == catalog.StateBroken && state == catalog.StateOK {
				state = catalog.StateDegraded
			}
		}
		return kit.Outcome(done, name, state, o.Detail)
	}
	return kit.Outcome(done, name, catalog.StateOK, "")
}
