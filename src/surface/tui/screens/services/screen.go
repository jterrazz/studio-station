package servicesscreen

import (
	"context"
	"strconv"

	tea "charm.land/bubbletea/v2"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/core/call"
	"github.com/jterrazz/studio-station/src/core/machine"
	"github.com/jterrazz/studio-station/src/page"
	pageports "github.com/jterrazz/studio-station/src/pages/ports"
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
	tab.View[ports.Listing]
	ctx context.Context
	d   Deps
}

func New(ctx context.Context, d Deps, here page.Context, describe tab.Describe) Model {
	source := func(ctx context.Context, _ bool) (<-chan ports.Listing, error) {
		f, err := call.Invoke(ctx, d.Calls, d.Target, portscalls.List, portscalls.ListIn{AllMachines: true}, nil)
		if err != nil {
			return nil, err
		}
		out := make(chan ports.Listing, 1)
		out <- f
		close(out)
		return out, nil
	}
	build := func(l ports.Listing) page.Page { return pageports.List(l, here) }
	return Model{View: tab.NewView(ID, "Services", "services", here, source, build, describe), ctx: ctx, d: d}
}

func (m Model) Update(msg tea.Msg) (tab.Screen, tea.Cmd) {
	view, cmd, ev := m.View.Update(msg)
	m.View = view
	if ev == nil || ev.Action.Key != pageports.KeyStopSharing {
		return m, cmd
	}
	for _, h := range m.Last.Fleet.Hosts {
		for _, s := range h.Services {
			if pageports.Row(s).ID != ev.Row {
				continue
			}
			host := ""
			if !h.Here {
				host = h.Name
			}
			name := ev.Row
			in := portscalls.StopSharingIn{VM: s.VM, Port: strconv.Itoa(s.GuestPort), Host: host}
			run := tab.Confirmed(ID, pageports.StopConfirm(s.VM, s.GuestPort), func(answer string) (tab.Status, error) {
				in.Confirm = answer
				_, err := call.Invoke(m.ctx, m.d.Calls, m.d.Target, portscalls.StopSharing, in, nil)
				return tab.Outcome("not shared", name, catalog.StateOK, ""), err
			})
			var act tea.Cmd
			m.View, act = m.Act(run)
			return m, tea.Batch(cmd, act)
		}
	}
	return m, cmd
}
