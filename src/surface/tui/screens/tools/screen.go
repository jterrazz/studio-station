package toolsscreen

import (
	"context"

	tea "charm.land/bubbletea/v2"

	"github.com/jterrazz/studio-station/src/apps"
	appscalls "github.com/jterrazz/studio-station/src/apps/calls"
	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/core/call"
	"github.com/jterrazz/studio-station/src/core/machine"
	"github.com/jterrazz/studio-station/src/page"
	pageapps "github.com/jterrazz/studio-station/src/pages/apps"
	"github.com/jterrazz/studio-station/src/surface/tui/screens/tab"
)

const ID = "tools"

type Deps struct {
	Calls  call.Router
	Target machine.Ref
	// value: drops the cached reading, so a fresh one reads again.
	Forget func()
}

type Model struct {
	tab.View[[]apps.Report]
	ctx context.Context
	d   Deps
}

func New(ctx context.Context, d Deps, here page.Context, describe tab.Describe) Model {
	source := func(ctx context.Context, fresh bool) (<-chan []apps.Report, error) {
		if fresh {
			d.Forget()
		}
		return tab.Values(call.Follow(ctx, d.Calls, d.Target, appscalls.List, appscalls.ListIn{}, nil))
	}
	build := func(r []apps.Report) page.Page { return pageapps.List(r, here, false) }
	return Model{View: tab.NewView(ID, "Tools", "tools", here, source, build, describe), ctx: ctx, d: d}
}

func (m Model) Update(msg tea.Msg) (tab.Screen, tea.Cmd) {
	view, cmd, ev := m.View.Update(msg)
	m.View = view
	if ev == nil || ev.Row == "" {
		return m, cmd
	}
	key, name := ev.Action.Key, ev.Row
	verb := key
	if ev.Action.Confirm != nil {
		verb = ev.Action.Confirm.Yes
	}
	run := tab.Confirmed(ID, pageapps.Confirm(verb, name), func(answer string) (tab.Status, error) { return m.run(key, name, answer) })
	var act tea.Cmd
	m.View, act = m.Act(run)
	return m, tea.Batch(cmd, act)
}

func (m Model) run(key, name, answer string) (tab.Status, error) {
	names := []string{name}
	switch key {
	case pageapps.KeyInstall:
		outcomes, err := call.Invoke(m.ctx, m.d.Calls, m.d.Target, appscalls.Install, appscalls.InstallIn{Names: names, Interactive: true}, nil)
		return outcome("installed", name, outcomes), err
	case pageapps.KeyRemove:
		outcomes, err := call.Invoke(m.ctx, m.d.Calls, m.d.Target, appscalls.Remove, appscalls.RemoveIn{Names: names, Confirm: answer}, nil)
		return outcome("removed", name, outcomes), err
	}
	outcomes, err := call.Invoke(m.ctx, m.d.Calls, m.d.Target, appscalls.Update, appscalls.UpdateIn{Names: names}, nil)
	return outcome("updated", name, outcomes), err
}

// value: the status bar names the tool asked for; a Then step that failed
// degrades the line rather than hiding behind the tool's own success.
func outcome(done, name string, outcomes []apps.Outcome) tab.Status {
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
		return tab.Outcome(done, name, state, o.Detail)
	}
	return tab.Outcome(done, name, catalog.StateOK, "")
}
