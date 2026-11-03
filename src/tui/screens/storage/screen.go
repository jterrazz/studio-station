package storagescreen

import (
	"context"

	tea "charm.land/bubbletea/v2"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/domain/storage"
	"github.com/jterrazz/studio-station/src/page"
	pagestorage "github.com/jterrazz/studio-station/src/pages/storage"
	"github.com/jterrazz/studio-station/src/tui/screens/tab"
)

const ID = "storage"

type Storage interface {
	Watch(ctx context.Context) <-chan storage.Report
	Forget()
	Clean(ctx context.Context, names []string) ([]storage.Outcome, error)
	Reset(ctx context.Context, names []string) ([]storage.Outcome, error)
	Plan(names []string) []storage.PathReport
}

type Model struct {
	tab.View[storage.Report]
	ctx     context.Context
	storage Storage
}

func New(ctx context.Context, s Storage, here page.Context, describe tab.Describe) Model {
	source := func(ctx context.Context, fresh bool) (<-chan storage.Report, error) {
		if fresh {
			s.Forget()
		}
		return s.Watch(ctx), nil
	}
	build := func(r storage.Report) page.Page { return pagestorage.List(r, here) }
	return Model{View: tab.NewView(ID, "Storage", "data stored", here, source, build, describe), ctx: ctx, storage: s}
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
		m.View, act = m.Act(nil, m.exec("cleaned", name, m.storage.Clean))
	case pagestorage.KeyReset:
		c := pagestorage.ResetConfirm([]string{name}, m.storage.Plan([]string{name}))
		m.View, act = m.Act(&c, m.exec("reset", name, m.storage.Reset))
	}
	return m, tea.Batch(cmd, act)
}

func (m Model) exec(verb, name string, act func(context.Context, []string) ([]storage.Outcome, error)) tea.Cmd {
	return tab.Exec(ID, func() (tab.Status, error) {
		outcomes, err := act(m.ctx, []string{name})
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
