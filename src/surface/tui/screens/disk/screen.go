package diskscreen

import (
	"context"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/core/call"
	"github.com/jterrazz/studio-station/src/core/machine"
	"github.com/jterrazz/studio-station/src/disk"
	diskcalls "github.com/jterrazz/studio-station/src/disk/calls"
	"github.com/jterrazz/studio-station/src/page"
	pagedisk "github.com/jterrazz/studio-station/src/pages/disk"
	"github.com/jterrazz/studio-station/src/surface/tui/screens/tab"
)

const ID = "disk"

type Deps struct {
	Calls  call.Router
	Target machine.Ref
	Forget func()
	Plan   func(app string) []disk.PathReport
}

type Model struct {
	tab.View[disk.Reading]
	ctx context.Context
	d   Deps
}

func New(ctx context.Context, d Deps, here page.Context, describe tab.Describe) Model {
	source := func(ctx context.Context, fresh bool) (<-chan disk.Reading, error) {
		if fresh {
			d.Forget()
		}
		return tab.Values(call.Follow(ctx, d.Calls, d.Target, diskcalls.Read, diskcalls.ReadIn{}, nil))
	}
	build := func(r disk.Reading) page.Page { return pagedisk.Read(r, here) }
	return Model{View: tab.NewView(ID, "Storage", "data stored", here, source, build, describe), ctx: ctx, d: d}
}

func (m Model) Update(msg tea.Msg) (tab.Screen, tea.Cmd) {
	view, cmd, ev := m.View.Update(msg)
	m.View = view
	if ev == nil || ev.Row == "" {
		return m, cmd
	}
	_, name, _ := strings.Cut(ev.Row, "/")
	var act tea.Cmd
	switch ev.Action.Key {
	case pagedisk.KeyMove:
		m.View, act = m.Act(m.exec("moved", name, page.Confirm{}, m.move))
	case pagedisk.KeyFree:
		m.View, act = m.Act(m.exec("freed", name, page.Confirm{}, m.free))
	case pagedisk.KeyErase:
		m.View, act = m.Act(m.exec("erased", name, pagedisk.EraseConfirm(name, m.d.Plan(name)), m.erase))
	}
	return m, tea.Batch(cmd, act)
}

func (m Model) move(ctx context.Context, folder, _ string) (disk.Outcome, error) {
	return call.Invoke(ctx, m.d.Calls, m.d.Target, diskcalls.Move, diskcalls.MoveIn{Folder: folder}, nil)
}

func (m Model) free(ctx context.Context, app, _ string) (disk.Outcome, error) {
	outcomes, err := call.Invoke(ctx, m.d.Calls, m.d.Target, diskcalls.Free, diskcalls.FreeIn{Apps: []string{app}}, nil)
	for _, o := range outcomes {
		if o.Name == app {
			return o, err
		}
	}
	return disk.Outcome{Name: app, Result: catalog.StateOK}, err
}

func (m Model) erase(ctx context.Context, app, answer string) (disk.Outcome, error) {
	return call.Invoke(ctx, m.d.Calls, m.d.Target, diskcalls.Erase, diskcalls.EraseIn{App: app, Confirm: answer}, nil)
}

func (m Model) exec(verb, name string, question page.Confirm, act func(context.Context, string, string) (disk.Outcome, error)) tea.Cmd {
	return tab.Confirmed(ID, question, func(answer string) (tab.Status, error) {
		o, err := act(m.ctx, name, answer)
		detail := o.Detail
		if o.Result == catalog.StateOK || o.Result == "" {
			return tab.Outcome(verb, name, catalog.StateOK, ""), err
		}
		return tab.Outcome(verb, name, o.Result, detail), err
	})
}
