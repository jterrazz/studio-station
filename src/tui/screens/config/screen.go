package configscreen

import (
	"context"

	tea "charm.land/bubbletea/v2"

	"github.com/jterrazz/studio-station/src/domain/config"
	"github.com/jterrazz/studio-station/src/page"
	pageconfig "github.com/jterrazz/studio-station/src/pages/config"
	"github.com/jterrazz/studio-station/src/tui/screens/tab"
)

const ID = "config"

type Config interface {
	Watch(ctx context.Context) (<-chan []config.Report, error)
	Defaults(item string) map[string]string
	CheckInstall(names []string, interactive bool) error
	Install(ctx context.Context, name string, values map[string]string, interactive bool) (config.Change, error)
	CheckUninstall(names []string) error
	Uninstall(ctx context.Context, name string) (config.Change, error)
}

type Model struct {
	tab.View[[]config.Report]
	ctx    context.Context
	config Config
}

func New(ctx context.Context, c Config, here page.Context, describe tab.Describe) Model {
	source := func(watch context.Context, _ bool) (<-chan []config.Report, error) { return c.Watch(watch) }
	build := func(r []config.Report) page.Page { return pageconfig.Items(r, here) }
	return Model{View: tab.NewView(ID, "Config", "items", here, source, build, describe), ctx: ctx, config: c}
}

func (m Model) Update(msg tea.Msg) (tab.Screen, tea.Cmd) {
	view, cmd, ev := m.View.Update(msg)
	m.View = view
	if ev == nil || ev.Row == "" {
		return m, cmd
	}
	name := ev.Row
	var act tea.Cmd
	switch ev.Action.Key {
	case pageconfig.KeyInstall:
		var inputs bool
		for _, r := range m.Last {
			if r.Item.Name == name && len(r.Item.Inputs) > 0 {
				inputs = true
				m.View, act = m.Fill(pageconfig.InputsForm(name, r.Item.Inputs, m.config.Defaults(name)), func(values map[string]string) tea.Cmd {
					return m.start(m.install(name, values))
				})
			}
		}
		if !inputs {
			m.View, act = m.Act(nil, m.install(name, nil))
		}
	case pageconfig.KeyUninstall:
		c := pageconfig.UninstallConfirm(name)
		m.View, act = m.Act(&c, m.uninstall(name))
	}
	return m, tea.Batch(cmd, act)
}

func (m Model) start(run tea.Cmd) tea.Cmd {
	return func() tea.Msg { return tab.Begin(ID, run) }
}

func (m Model) install(name string, values map[string]string) tea.Cmd {
	return tab.Exec(ID, func() (tab.Status, error) {
		if err := m.config.CheckInstall([]string{name}, true); err != nil {
			return tab.Status{}, err
		}
		change, err := m.config.Install(m.ctx, name, values, true)
		return outcome("installed", change), err
	})
}

func (m Model) uninstall(name string) tea.Cmd {
	return tab.Exec(ID, func() (tab.Status, error) {
		if err := m.config.CheckUninstall([]string{name}); err != nil {
			return tab.Status{}, err
		}
		change, err := m.config.Uninstall(m.ctx, name)
		return outcome("uninstalled", change), err
	})
}

func outcome(done string, c config.Change) tab.Status {
	detail := ""
	for _, s := range c.Steps {
		if s.State == c.Result {
			detail = s.Detail
		}
	}
	return tab.Outcome(done, c.Item, c.Result, detail)
}
