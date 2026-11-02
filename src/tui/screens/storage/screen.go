package storagescreen

import (
	"context"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/domain/storage"
	"github.com/jterrazz/studio-station/src/present"
	presentstorage "github.com/jterrazz/studio-station/src/present/storage"
	"github.com/jterrazz/studio-station/src/theme"
	"github.com/jterrazz/studio-station/src/tui/kit"
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
	ctx     context.Context
	storage Storage
	list    kit.ListScreen
}

func New(ctx context.Context, s Storage, describe kit.Describe, th theme.Theme) Model {
	m := Model{ctx: ctx, storage: s}
	m.list = kit.NewList(kit.ListConfig{
		ID:        ID,
		Watch:     m.watch,
		Describe:  describe,
		Empty:     "nothing stored",
		EmptyHint: "press r to measure again",
		Keys: []key.Binding{
			kit.Bind("c", "clean", "c"),
			kit.Bind("x", "reset", "x"),
			kit.FilterKey(),
		},
	}, th)
	return m
}

func (m Model) watch(ctx context.Context, fresh bool) (<-chan []kit.Group, error) {
	if fresh {
		m.storage.Forget()
	}
	return kit.Groups(ctx, m.storage.Watch(ctx), groups), nil
}

func groups(report storage.Report) []kit.Group {
	var groups []kit.Group
	for _, g := range presentstorage.Groups(report) {
		group := kit.Group{Title: g.Title, Rows: make([]kit.Row, 0, len(g.Rows))}
		for _, r := range g.Rows {
			group.Rows = append(group.Rows, kit.Row{Key: r.Entry.Name, Row: presentstorage.Row(r), Data: r})
		}
		groups = append(groups, group)
	}
	if len(report.Strays) > 0 {
		group := kit.Group{Title: "Strays"}
		for i, row := range presentstorage.StrayRows(report.Strays) {
			group.Rows = append(group.Rows, kit.Row{Key: report.Strays[i].Name, Row: row})
		}
		groups = append(groups, group)
	}
	return groups
}

func (m Model) Init() tea.Cmd { return m.list.Init() }

func (m Model) Focus() kit.Focus { return m.list.Focus() }

func (m Model) Busy() bool { return m.list.Busy() }

func (m Model) Loading() bool { return m.list.Loading() }

func (m Model) Keys() []key.Binding { return m.list.Keys() }

func (m Model) View(width, height int) string { return m.list.View(width, height) }

func (m Model) Update(msg tea.Msg) (kit.Screen, tea.Cmd) {
	if k, ok := msg.(tea.KeyPressMsg); ok && m.list.Focus() == kit.FocusNormal && (k.String() == "c" || k.String() == "x") {
		row, found := m.list.Current()
		r, isRow := row.Data.(storage.Row)
		if !found || !isRow || m.list.Busy() {
			return m, nil
		}
		if k.String() == "c" {
			return m, m.clean(row.Key)
		}
		if len(r.Entry.Storage.Reset) == 0 {
			return m, kit.Say(kit.Status{Mark: present.MarkAbsent, Text: r.Entry.Storage.NoReset})
		}
		var cmd tea.Cmd
		m.list, cmd = m.list.Open(kit.ConfirmReset("Reset "+row.Key+"?", m.plan(row.Key), m.reset(row.Key)))
		return m, cmd
	}
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m Model) plan(name string) string {
	var lines []string
	for _, p := range m.storage.Plan([]string{name}) {
		lines = append(lines, string(p.Level)+"  "+p.Path+"  "+presentstorage.Size(p.Bytes))
	}
	return strings.Join(append(lines, "installed files and kept files stay"), "\n")
}

func (m Model) clean(name string) tea.Cmd {
	return m.exec("cleaned", name, m.storage.Clean)
}

func (m Model) reset(name string) tea.Cmd {
	return m.exec("reset", name, m.storage.Reset)
}

func (m Model) exec(verb, name string, act func(context.Context, []string) ([]storage.Outcome, error)) tea.Cmd {
	return kit.Exec(ID, func() (kit.Status, error) {
		outcomes, err := act(m.ctx, []string{name})
		for _, o := range outcomes {
			if o.Name == name {
				return kit.Outcome(verb, name, o.Result, presentstorage.OutcomeDetail(o)), err
			}
		}
		return kit.Outcome(verb, name, catalog.StateOK, ""), err
	})
}
