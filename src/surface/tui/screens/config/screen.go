package configscreen

import (
	"context"

	tea "charm.land/bubbletea/v2"

	"github.com/jterrazz/studio-station/src/core/call"
	"github.com/jterrazz/studio-station/src/core/confirm"
	"github.com/jterrazz/studio-station/src/core/machine"
	"github.com/jterrazz/studio-station/src/page"
	pageconfig "github.com/jterrazz/studio-station/src/pages/config"
	"github.com/jterrazz/studio-station/src/settings"
	settingscalls "github.com/jterrazz/studio-station/src/settings/calls"
	"github.com/jterrazz/studio-station/src/surface/tui/screens/tab"
)

const ID = "config"

type Deps struct {
	Calls  call.Router
	Target machine.Ref
	// value: the values an item's form starts from.
	Defaults func(item string) map[string]string
}

type Model struct {
	tab.View[[]settings.Report]
	ctx context.Context
	d   Deps
}

func New(ctx context.Context, d Deps, here page.Context, describe tab.Describe) Model {
	source := func(watch context.Context, _ bool) (<-chan []settings.Report, error) {
		return tab.Values(call.Follow(watch, d.Calls, d.Target, settingscalls.List, settingscalls.ListIn{}, nil))
	}
	build := func(r []settings.Report) page.Page { return pageconfig.Items(r, here) }
	return Model{View: tab.NewView(ID, "Config", "items", here, source, build, describe), ctx: ctx, d: d}
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
				m.View, act = m.Fill(pageconfig.InputsForm(name, r.Item.Inputs, m.d.Defaults(name)), func(values map[string]string) tea.Cmd {
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
		in := settingscalls.InstallIn{Names: []string{name}, Interactive: true}
		if values != nil {
			in.Values = map[string]map[string]string{name: values}
		}
		changes, err := call.Invoke(m.ctx, m.d.Calls, m.d.Target, settingscalls.Install, in, nil)
		return outcome("installed", name, changes), err
	})
}

func (m Model) uninstall(name string) tea.Cmd {
	return tab.Exec(ID, func() (tab.Status, error) {
		var changes []settings.Change
		err := confirm.Answered(func(answer string) (err error) {
			changes, err = call.Invoke(m.ctx, m.d.Calls, m.d.Target, settingscalls.Uninstall, settingscalls.UninstallIn{Names: []string{name}, Confirm: answer}, nil)
			return err
		})
		return outcome("uninstalled", name, changes), err
	})
}

func outcome(done, name string, changes []settings.Change) tab.Status {
	c := settings.Change{Item: name}
	if len(changes) > 0 {
		c = changes[0]
	}
	detail := ""
	for _, s := range c.Steps {
		if s.State == c.Result {
			detail = s.Detail
		}
	}
	return tab.Outcome(done, c.Item, c.Result, detail)
}
