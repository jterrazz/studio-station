package homescreen

import (
	"context"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/jterrazz/studio-station/src/page"
	pagehome "github.com/jterrazz/studio-station/src/pages/home"
	"github.com/jterrazz/studio-station/src/surface/tui/screens/tab"
)

const (
	ID      = "home"
	every   = 2 * time.Second
	history = 16
)

type Status interface {
	Watch(ctx context.Context) <-chan pagehome.Overview
	Forget()
	Sample(ctx context.Context) (pagehome.Sample, error)
}

type samples struct{ list []pagehome.Sample }

type Fix func(ctx context.Context, words []string) error

type Model struct {
	tab.View[pagehome.Overview]
	ctx     context.Context
	status  Status
	here    page.Context
	live    *samples
	visible bool
	ticking bool
	fix     Fix
}

type sampled struct {
	sample pagehome.Sample
	err    error
}

type tick struct{}

func New(ctx context.Context, s Status, fix Fix, here page.Context, describe tab.Describe) Model {
	live := &samples{}
	source := func(ctx context.Context, fresh bool) (<-chan pagehome.Overview, error) {
		if fresh {
			s.Forget()
		}
		return s.Watch(ctx), nil
	}
	build := func(snap pagehome.Overview) page.Page {
		list := live.list
		if len(list) == 0 && !snap.System.Sample.At.IsZero() {
			list = []pagehome.Sample{snap.System.Sample}
		}
		return pagehome.Live(pagehome.Snapshot(snap, here, false), list, snap)
	}
	return Model{View: tab.NewView(ID, "Home", "actions", here, source, build, describe), ctx: ctx, status: s, here: here, live: live, visible: true, ticking: true, fix: fix}
}

func (m Model) Init() tea.Cmd { return tea.Batch(m.View.Init(), next()) }

func next() tea.Cmd { return tea.Tick(every, func(time.Time) tea.Msg { return tick{} }) }

func (m Model) measure() tea.Cmd {
	return func() tea.Msg {
		sample, err := m.status.Sample(m.ctx)
		return sampled{sample: sample, err: err}
	}
}

func (m Model) Update(msg tea.Msg) (tab.Screen, tea.Cmd) {
	switch msg := msg.(type) {
	case sampled:
		if msg.err != nil {
			return m, nil
		}
		m.live.list = append(m.live.list, msg.sample)
		if len(m.live.list) > history {
			m.live.list = m.live.list[len(m.live.list)-history:]
		}
		snap := m.Last
		m.Browser = m.Browser.Show(pagehome.Live(pagehome.Snapshot(snap, m.here, false), m.live.list, snap))
		return m, nil
	case tick:
		if !m.visible {
			m.ticking = false
			return m, nil
		}
		return m, tea.Batch(m.measure(), next())
	case tab.Shown:
		m.View, _, _ = m.View.Update(msg)
		m.visible = msg.Visible
		if m.visible && !m.ticking {
			m.ticking = true
			return m, tea.Batch(m.measure(), next())
		}
		return m, nil
	}
	view, cmd, ev := m.View.Update(msg)
	m.View = view
	if ev == nil || ev.Action.Key != pagehome.KeyFix || m.fix == nil {
		return m, cmd
	}
	words := fixOf(m.Browser.Page, ev.Row)
	if len(words) == 0 {
		return m, cmd
	}
	ctx, fix := m.ctx, m.fix
	run := tab.Exec(ID, func() (tab.Status, error) {
		if err := fix(ctx, words); err != nil {
			return tab.Status{}, err
		}
		return tab.Status{Mark: page.OK, Text: "ran station " + strings.Join(words, " ")}, nil
	})
	var act tea.Cmd
	m.View, act = m.Act(nil, run)
	return m, tea.Batch(cmd, act)
}

// invariant: the words after `station` of the action's hint; an External one
// has none, so enter only ever runs station itself.
func fixOf(p page.Page, row string) []string {
	for _, s := range p.Sections {
		if s.Role != page.Actions {
			continue
		}
		for _, it := range s.Items {
			if it.Row != nil && it.Row.ID == row && it.Row.Hint != nil && !it.Row.Hint.External {
				return strings.Fields(strings.TrimPrefix(it.Row.Hint.Command, "station "))
			}
		}
	}
	return nil
}
