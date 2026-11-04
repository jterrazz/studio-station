package storagescreen

import (
	"context"

	tea "charm.land/bubbletea/v2"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/core/call"
	"github.com/jterrazz/studio-station/src/core/machine"
	"github.com/jterrazz/studio-station/src/disk"
	diskcalls "github.com/jterrazz/studio-station/src/disk/calls"
	"github.com/jterrazz/studio-station/src/page"
	pagestorage "github.com/jterrazz/studio-station/src/pages/storage"
	"github.com/jterrazz/studio-station/src/surface/tui/screens/tab"
)

const ID = "storage"

type Deps struct {
	Calls  call.Router
	Target machine.Ref
	Forget func()
	Plan   func(names []string) []disk.PathReport
}

type Model struct {
	tab.View[disk.Report]
	ctx context.Context
	d   Deps
}

func New(ctx context.Context, d Deps, here page.Context, describe tab.Describe) Model {
	source := func(ctx context.Context, fresh bool) (<-chan disk.Report, error) {
		if fresh {
			d.Forget()
		}
		return tab.Values(call.Follow(ctx, d.Calls, d.Target, diskcalls.List, diskcalls.ListIn{}, nil))
	}
	build := func(r disk.Report) page.Page { return pagestorage.List(r, here) }
	return Model{View: tab.NewView(ID, "Storage", "data stored", here, source, build, describe), ctx: ctx, d: d}
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
	case pagestorage.KeyClean:
		m.View, act = m.Act(m.exec("cleaned", name, page.Confirm{}, m.clean))
	case pagestorage.KeyReset:
		m.View, act = m.Act(m.exec("reset", name, pagestorage.ResetConfirm([]string{name}, m.d.Plan([]string{name})), m.reset))
	}
	return m, tea.Batch(cmd, act)
}

func (m Model) clean(ctx context.Context, names []string, _ string) ([]disk.Outcome, error) {
	return call.Invoke(ctx, m.d.Calls, m.d.Target, diskcalls.Clean, diskcalls.CleanIn{Names: names}, nil)
}

func (m Model) reset(ctx context.Context, names []string, answer string) ([]disk.Outcome, error) {
	return call.Invoke(ctx, m.d.Calls, m.d.Target, diskcalls.Reset, diskcalls.ResetIn{Names: names, Confirm: answer}, nil)
}

func (m Model) exec(verb, name string, question page.Confirm, act func(context.Context, []string, string) ([]disk.Outcome, error)) tea.Cmd {
	return tab.Confirmed(ID, question, func(answer string) (tab.Status, error) {
		outcomes, err := act(m.ctx, []string{name}, answer)
		for _, o := range outcomes {
			if o.Name == name {
				detail := o.Detail
				if o.Result == catalog.StateOK {
					detail = ""
				}
				return tab.Outcome(verb, name, o.Result, detail), err
			}
		}
		return tab.Outcome(verb, name, catalog.StateOK, ""), err
	})
}
