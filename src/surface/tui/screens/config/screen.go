package configscreen

import (
	"context"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/jterrazz/studio-station/src/core/call"
	"github.com/jterrazz/studio-station/src/core/machine"
	"github.com/jterrazz/studio-station/src/page"
	pagesettings "github.com/jterrazz/studio-station/src/pages/settings"
	"github.com/jterrazz/studio-station/src/settings"
	settingscalls "github.com/jterrazz/studio-station/src/settings/calls"
	"github.com/jterrazz/studio-station/src/surface/tui/screens/tab"
)

const ID = "config"

type Deps struct {
	Calls  call.Router
	Target machine.Ref
	// value: the values a setting's form starts from.
	Defaults func(item string) map[string]string
	Now      func() time.Time
}

type Model struct {
	tab.View[[]settings.Row]
	ctx context.Context
	d   Deps
}

func New(ctx context.Context, d Deps, here page.Context, describe tab.Describe) Model {
	source := func(watch context.Context, _ bool) (<-chan []settings.Row, error) {
		return tab.Values(call.Follow(watch, d.Calls, d.Target, settingscalls.List, settingscalls.ListIn{}, nil))
	}
	build := func(r []settings.Row) page.Page { return pagesettings.List(r, d.Now(), false, here) }
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
	case pagesettings.KeyApply:
		var inputs bool
		for _, r := range m.Last {
			if r.Item().Name == name && len(r.Item().Inputs) > 0 {
				inputs = true
				m.View, act = m.Fill(pagesettings.InputsForm(name, r.Item().Inputs, m.d.Defaults(name)), func(values map[string]string) tea.Cmd {
					return m.start(m.install(name, values))
				})
			}
		}
		if !inputs {
			m.View, act = m.Act(m.install(name, nil))
		}
	case pagesettings.KeyOff:
		m.View, act = m.Act(m.off(name))
	}
	return m, tea.Batch(cmd, act)
}

func (m Model) start(run tea.Cmd) tea.Cmd {
	return func() tea.Msg { return tab.Begin(ID, run) }
}

func (m Model) install(name string, values map[string]string) tea.Cmd {
	return tab.Exec(ID, func() (tab.Status, error) {
		in := settingscalls.ApplyIn{Names: []string{name}, Interactive: true}
		if values != nil {
			in.Values = map[string]map[string]string{name: values}
		}
		changes, err := call.Invoke(m.ctx, m.d.Calls, m.d.Target, settingscalls.Apply, in, nil)
		return outcome("applied", name, changes), err
	})
}

func (m Model) off(name string) tea.Cmd {
	return tab.Confirmed(ID, pagesettings.OffConfirm([]string{name}), func(answer string) (tab.Status, error) {
		changes, err := call.Invoke(m.ctx, m.d.Calls, m.d.Target, settingscalls.Off, settingscalls.OffIn{Names: []string{name}, Confirm: answer}, nil)
		return outcome("off here", name, changes), err
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
