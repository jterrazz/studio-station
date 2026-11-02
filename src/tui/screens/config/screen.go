package configscreen

import (
	"context"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/jterrazz/studio-station/src/domain/config"
	"github.com/jterrazz/studio-station/src/present"
	presentconfig "github.com/jterrazz/studio-station/src/present/config"
	"github.com/jterrazz/studio-station/src/theme"
	"github.com/jterrazz/studio-station/src/tui/kit"
)

const ID = "config"

type Config interface {
	List(ctx context.Context) ([]config.Report, error)
	Defaults(item string) map[string]string
	CheckInstall(names []string, interactive bool) error
	Install(ctx context.Context, name string, values map[string]string, interactive bool) (config.Change, error)
	CheckUninstall(names []string) error
	Uninstall(ctx context.Context, name string) (config.Change, error)
}

type Model struct {
	ctx    context.Context
	config Config
	list   kit.ListScreen
}

func New(ctx context.Context, c Config, describe kit.Describe, th theme.Theme) Model {
	m := Model{ctx: ctx, config: c}
	m.list = kit.NewList(kit.ListConfig{
		ID:        ID,
		Load:      m.load,
		Describe:  describe,
		Empty:     "none",
		EmptyHint: "no config item is placed on this machine",
		Detail: func(r kit.Row) (string, present.Document, bool) {
			report, ok := r.Data.(config.Report)
			return r.Key, presentconfig.Detail(report), ok
		},
		Keys: []key.Binding{
			kit.Bind("i", "install", "i"),
			kit.Bind("u", "uninstall", "u"),
			kit.FilterKey(),
			kit.DetailKey(),
		},
	}, th)
	return m
}

func (m Model) load() ([]kit.Group, kit.Status, error) {
	reports, err := m.config.List(m.ctx)
	if err != nil {
		return nil, kit.Status{}, err
	}
	byName := make(map[string]config.Report, len(reports))
	for _, r := range reports {
		byName[r.Item.Name] = r
	}
	groups := kit.GroupsOf(presentconfig.Items(reports, "").Doc)
	for gi := range groups {
		for ri := range groups[gi].Rows {
			if r, ok := byName[groups[gi].Rows[ri].Key]; ok {
				groups[gi].Rows[ri].Data = r
			}
		}
	}
	return groups, kit.Status{}, nil
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
	if s := k.String(); s != "i" && s != "u" {
		return nil, false
	}
	row, ok := m.list.Current()
	report, isItem := row.Data.(config.Report)
	if !ok || !isItem || m.list.Busy() {
		return nil, true
	}
	name := row.Key
	var cmd tea.Cmd
	switch {
	case k.String() == "i" && len(report.Item.Inputs) > 0:
		m.list, cmd = m.list.Open(kit.Form("Configure "+name, report.Item.Inputs, m.config.Defaults(name), func(values map[string]string) tea.Cmd {
			return m.install(name, values)
		}))
	case k.String() == "i":
		m.list, cmd = m.list.Act(m.install(name, nil))
	case !report.Item.Reversible:
		cmd = kit.Say(kit.Status{Mark: present.MarkAbsent, Text: name + " is not reversible"})
	default:
		m.list, cmd = m.list.Open(kit.Confirm("Uninstall "+name+"?", m.uninstall(name)))
	}
	return cmd, true
}

func (m Model) install(name string, values map[string]string) tea.Cmd {
	return kit.Exec(ID, func() (kit.Status, error) {
		if err := m.config.CheckInstall([]string{name}, true); err != nil {
			return kit.Status{}, err
		}
		change, err := m.config.Install(m.ctx, name, values, true)
		return outcome("installed", change), err
	})
}

func (m Model) uninstall(name string) tea.Cmd {
	return kit.Exec(ID, func() (kit.Status, error) {
		if err := m.config.CheckUninstall([]string{name}); err != nil {
			return kit.Status{}, err
		}
		change, err := m.config.Uninstall(m.ctx, name)
		return outcome("uninstalled", change), err
	})
}

func outcome(done string, c config.Change) kit.Status {
	detail := ""
	for _, s := range c.Steps {
		if s.State == c.Result {
			detail = s.Detail
		}
	}
	return kit.Outcome(done, c.Item, c.Result, detail)
}
