package cleanscreen

import (
	"context"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/domain/clean"
	"github.com/jterrazz/studio-station/src/present"
	presentclean "github.com/jterrazz/studio-station/src/present/clean"
	"github.com/jterrazz/studio-station/src/theme"
	"github.com/jterrazz/studio-station/src/tui/kit"
)

const ID = "clean"

type Clean interface {
	List(ctx context.Context) ([]clean.Report, error)
	Clean(ctx context.Context, names []string) ([]clean.Outcome, error)
}

type Model struct {
	ctx   context.Context
	clean Clean
	list  kit.ListScreen
}

func New(ctx context.Context, c Clean, describe kit.Describe, th theme.Theme) Model {
	m := Model{ctx: ctx, clean: c}
	m.list = kit.NewList(kit.ListConfig{
		ID:        ID,
		Load:      m.load,
		Describe:  describe,
		Empty:     "nothing to clean",
		EmptyHint: "press r to measure again",
		Detail: func(r kit.Row) (string, present.Document, bool) {
			report, ok := r.Data.(clean.Report)
			return r.Key, presentclean.Detail(report), ok
		},
		Keys: []key.Binding{
			kit.Bind("c", "clean", "c"),
			kit.FilterKey(),
			kit.DetailKey(),
		},
	}, th)
	return m
}

func (m Model) load() ([]kit.Group, kit.Status, error) {
	reports, err := m.clean.List(m.ctx)
	if err != nil {
		return nil, kit.Status{}, err
	}
	if len(reports) == 0 {
		return nil, kit.Status{}, nil
	}
	rows := presentclean.Rows(reports)
	group := kit.Group{Rows: make([]kit.Row, 0, len(rows))}
	for i, r := range rows {
		group.Rows = append(group.Rows, kit.Row{Key: reports[i].Item.Name, Row: r, Data: reports[i]})
	}
	return []kit.Group{group}, kit.Status{}, nil
}

func (m Model) Init() tea.Cmd { return m.list.Init() }

func (m Model) Focus() kit.Focus { return m.list.Focus() }

func (m Model) Busy() bool { return m.list.Busy() }

func (m Model) Keys() []key.Binding { return m.list.Keys() }

func (m Model) View(width, height int) string { return m.list.View(width, height) }

func (m Model) Update(msg tea.Msg) (kit.Screen, tea.Cmd) {
	if k, ok := msg.(tea.KeyPressMsg); ok && m.list.Focus() == kit.FocusNormal && k.String() == "c" {
		row, found := m.list.Current()
		report, isReport := row.Data.(clean.Report)
		if !found || !isReport || m.list.Busy() {
			return m, nil
		}
		if !report.Available {
			return m, kit.Say(kit.Status{Mark: present.MarkAbsent, Text: row.Key + " is not available here"})
		}
		var cmd tea.Cmd
		m.list, cmd = m.list.Open(kit.Confirm("Reclaim "+row.Key+" ("+present.Bytes(report.Bytes)+")?", m.reclaim(row.Key)))
		return m, cmd
	}
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m Model) reclaim(name string) tea.Cmd {
	return kit.Exec(ID, func() (kit.Status, error) {
		outcomes, err := m.clean.Clean(m.ctx, []string{name})
		for _, o := range outcomes {
			if o.Name == name {
				return kit.Outcome("cleaned", name, o.Result, o.Detail), err
			}
		}
		return kit.Outcome("cleaned", name, catalog.StateOK, ""), err
	})
}
