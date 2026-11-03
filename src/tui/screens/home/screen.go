package homescreen

import (
	"context"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/jterrazz/studio-station/src/domain/status"
	"github.com/jterrazz/studio-station/src/page"
	pagehome "github.com/jterrazz/studio-station/src/pages/home"
	"github.com/jterrazz/studio-station/src/tui/screens/tab"
)

const (
	ID      = "home"
	every   = 2 * time.Second
	history = 16
)

type Status interface {
	Watch(ctx context.Context) <-chan status.Snapshot
	Forget()
	Sample(ctx context.Context) (status.Sample, error)
}

type samples struct{ list []status.Sample }

type Model struct {
	tab.View[status.Snapshot]
	ctx     context.Context
	status  Status
	here    page.Context
	live    *samples
	visible bool
	ticking bool
}

type sampled struct {
	sample status.Sample
	err    error
}

type tick struct{}

func New(ctx context.Context, s Status, here page.Context, describe tab.Describe) Model {
	live := &samples{}
	source := func(ctx context.Context, fresh bool) (<-chan status.Snapshot, error) {
		if fresh {
			s.Forget()
		}
		return s.Watch(ctx), nil
	}
	build := func(snap status.Snapshot) page.Page {
		list := live.list
		if len(list) == 0 && !snap.System.Sample.At.IsZero() {
			list = []status.Sample{snap.System.Sample}
		}
		return pagehome.Live(pagehome.Snapshot(snap, here, false), list, snap.System.MemoryTotal)
	}
	return Model{View: tab.NewView(ID, "Home", "actions", here, source, build, describe), ctx: ctx, status: s, here: here, live: live, visible: true, ticking: true}
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
		m.Browser = m.Browser.Show(pagehome.Live(pagehome.Snapshot(snap, m.here, false), m.live.list, snap.System.MemoryTotal))
		return m, nil
	case tick:
		if !m.visible {
			m.ticking = false
			return m, nil
		}
		return m, tea.Batch(m.measure(), next())
	case tab.Shown:
		m.visible = msg.Visible
		if m.visible && !m.ticking {
			m.ticking = true
			return m, tea.Batch(m.measure(), next())
		}
		return m, nil
	}
	view, cmd, _ := m.View.Update(msg)
	m.View = view
	return m, cmd
}
