package toolsscreen

import (
	"context"

	tea "charm.land/bubbletea/v2"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/domain/tools"
	"github.com/jterrazz/studio-station/src/page"
	pagetools "github.com/jterrazz/studio-station/src/pages/tools"
	"github.com/jterrazz/studio-station/src/surface/tui/screens/tab"
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
	tab.View[[]tools.Report]
	ctx   context.Context
	tools Tools
}

func New(ctx context.Context, t Tools, here page.Context, describe tab.Describe) Model {
	source := func(ctx context.Context, fresh bool) (<-chan []tools.Report, error) {
		if fresh {
			t.Forget()
		}
		return t.Watch(ctx), nil
	}
	build := func(r []tools.Report) page.Page { return pagetools.Catalogue(r, here) }
	return Model{View: tab.NewView(ID, "Tools", "tools", here, source, build, describe), ctx: ctx, tools: t}
}

func (m Model) Update(msg tea.Msg) (tab.Screen, tea.Cmd) {
	view, cmd, ev := m.View.Update(msg)
	m.View = view
	if ev == nil || ev.Row == "" {
		return m, cmd
	}
	key, name := ev.Action.Key, ev.Row
	run := tab.Exec(ID, func() (tab.Status, error) { return m.run(key, name) })
	var confirm *page.Confirm
	if ev.Action.Confirm != nil {
		c := pagetools.Confirm(ev.Action.Confirm.Yes, name)
		confirm = &c
	}
	var act tea.Cmd
	m.View, act = m.Act(confirm, run)
	return m, tea.Batch(cmd, act)
}

func (m Model) run(key, name string) (tab.Status, error) {
	switch key {
	case pagetools.KeyInstall:
		if err := m.tools.CheckInstall(m.ctx, []string{name}); err != nil {
			return tab.Status{}, err
		}
		outcomes, err := m.tools.Install(m.ctx, tools.InstallRequest{Names: []string{name}, Interactive: true})
		return outcome("installed", name, outcomes), err
	case pagetools.KeyUninstall:
		if err := m.tools.CheckUninstall([]string{name}); err != nil {
			return tab.Status{}, err
		}
		outcomes, err := m.tools.Uninstall(m.ctx, []string{name})
		return outcome("uninstalled", name, outcomes), err
	}
	if err := m.tools.CheckUpgrade([]string{name}); err != nil {
		return tab.Status{}, err
	}
	outcomes, err := m.tools.Upgrade(m.ctx, tools.UpgradeRequest{Names: []string{name}})
	return outcome("upgraded", name, outcomes), err
}

// value: the status bar names the tool asked for; a Then step that failed
// degrades the line rather than hiding behind the tool's own success.
func outcome(done, name string, outcomes []tools.Outcome) tab.Status {
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
