package servicesscreen

import (
	"context"
	"strconv"

	tea "charm.land/bubbletea/v2"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/core/call"
	"github.com/jterrazz/studio-station/src/core/machine"
	"github.com/jterrazz/studio-station/src/page"
	pageservices "github.com/jterrazz/studio-station/src/pages/services"
	"github.com/jterrazz/studio-station/src/ports"
	portscalls "github.com/jterrazz/studio-station/src/ports/calls"
	"github.com/jterrazz/studio-station/src/surface/tui/screens/tab"
)

const ID = "services"

type Deps struct {
	Calls  call.Router
	Target machine.Ref
}

type Model struct {
	tab.View[ports.Fleet]
	ctx context.Context
	d   Deps
}

func New(ctx context.Context, d Deps, here page.Context, describe tab.Describe) Model {
	source := func(ctx context.Context, _ bool) (<-chan ports.Fleet, error) {
		f, err := call.Invoke(ctx, d.Calls, d.Target, portscalls.List, portscalls.ListIn{}, nil)
		if err != nil {
			return nil, err
		}
		out := make(chan ports.Fleet, 1)
		out <- f
		close(out)
		return out, nil
	}
	build := func(f ports.Fleet) page.Page { return pageservices.Fleet(f, here) }
	return Model{View: tab.NewView(ID, "Services", "services", here, source, build, describe), ctx: ctx, d: d}
}

func (m Model) Update(msg tea.Msg) (tab.Screen, tea.Cmd) {
	view, cmd, ev := m.View.Update(msg)
	m.View = view
	if ev == nil || ev.Action.Key != pageservices.KeyRelease {
		return m, cmd
	}
	for _, h := range m.Last.Hosts {
		for _, s := range h.Services {
			if pageservices.Row(s, h.Name).ID != ev.Row {
				continue
			}
			target, host := s.VM+":"+strconv.Itoa(s.GuestPort), ""
			if !h.Here {
				host = h.Name
			}
			name := ev.Row
			run := tab.Confirmed(ID, pageservices.ReleaseConfirm(s.VM, s.GuestPort), func(answer string) (tab.Status, error) {
				_, err := call.Invoke(m.ctx, m.d.Calls, m.d.Target, portscalls.Release, portscalls.ReleaseIn{Target: target, Host: host, Confirm: answer}, nil)
				return tab.Outcome("released", name, catalog.StateOK, ""), err
			})
			var act tea.Cmd
			m.View, act = m.Act(run)
			return m, tea.Batch(cmd, act)
		}
	}
	return m, cmd
}
