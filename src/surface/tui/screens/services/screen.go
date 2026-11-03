package servicesscreen

import (
	"context"

	tea "charm.land/bubbletea/v2"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/domain/services"
	"github.com/jterrazz/studio-station/src/page"
	pageservices "github.com/jterrazz/studio-station/src/pages/services"
	"github.com/jterrazz/studio-station/src/surface/tui/screens/tab"
)

const ID = "services"

type Services interface {
	List(ctx context.Context, hereOnly bool) (services.Fleet, error)
	Release(ctx context.Context, t services.Target, host string, yes bool) (services.Lease, error)
}

type Model struct {
	tab.View[services.Fleet]
	ctx      context.Context
	services Services
}

func New(ctx context.Context, s Services, here page.Context, describe tab.Describe) Model {
	source := func(ctx context.Context, _ bool) (<-chan services.Fleet, error) {
		f, err := s.List(ctx, false)
		if err != nil {
			return nil, err
		}
		out := make(chan services.Fleet, 1)
		out <- f
		close(out)
		return out, nil
	}
	build := func(f services.Fleet) page.Page { return pageservices.Fleet(f, here) }
	return Model{View: tab.NewView(ID, "Services", "services", here, source, build, describe), ctx: ctx, services: s}
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
			target, host := services.Target{VM: s.VM, Port: s.GuestPort}, ""
			if !h.Here {
				host = h.Name
			}
			name := ev.Row
			run := tab.Exec(ID, func() (tab.Status, error) {
				_, err := m.services.Release(m.ctx, target, host, true)
				return tab.Outcome("released", name, catalog.StateOK, ""), err
			})
			var confirm *page.Confirm
			if s.Pinned {
				c := pageservices.ReleaseConfirm(s.VM, s.GuestPort)
				confirm = &c
			}
			var act tea.Cmd
			m.View, act = m.Act(confirm, run)
			return m, tea.Batch(cmd, act)
		}
	}
	return m, cmd
}
