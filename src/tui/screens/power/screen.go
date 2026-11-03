package powerscreen

import (
	"context"
	"strconv"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/domain/power"
	"github.com/jterrazz/studio-station/src/page"
	pagepower "github.com/jterrazz/studio-station/src/pages/power"
	"github.com/jterrazz/studio-station/src/tui/screens/tab"
)

const ID = "power"

type Power interface {
	// value: the tab's pages: this host's own read first, then the daemon's
	// subscription while it answers, so a guard's end redraws the tab at once.
	Pages(ctx context.Context) <-chan page.Page
	Awake(ctx context.Context, r power.Request) (power.Awoken, error)
	Sleep(ctx context.Context) (power.Slept, error)
	Zone() *time.Location
}

type Model struct {
	tab.View[page.Page]
	ctx   context.Context
	power Power
	here  page.Context
}

func New(ctx context.Context, p Power, here page.Context, describe tab.Describe) Model {
	source := func(ctx context.Context, _ bool) (<-chan page.Page, error) { return p.Pages(ctx), nil }
	build := func(pg page.Page) page.Page { return pg }
	return Model{View: tab.NewView(ID, "Power", "power", here, source, build, describe), ctx: ctx, power: p, here: here}
}

func (m Model) Badge() page.Value {
	// value: the header's badge while a session holds: the Power section's
	// summary without its time left (awake, then its flags).
	for _, s := range m.Last.Sections {
		if s.ID != "power" || s.Role != page.Banner || len(s.Caption) == 0 || len(s.Caption[0].Items) < 2 {
			continue
		}
		items := s.Caption[0].Items
		return page.List(items[:len(items)-1]...)
	}
	return page.None()
}

func (m Model) Update(msg tea.Msg) (tab.Screen, tea.Cmd) {
	view, cmd, ev := m.View.Update(msg)
	m.View = view
	if ev == nil {
		return m, cmd
	}
	switch ev.Action.Key {
	case pagepower.KeyAwake:
		var fill tea.Cmd
		m.View, fill = m.Fill(pagepower.AwakeForm(m.here.Machine), func(values map[string]string) tea.Cmd {
			term, err := time.ParseDuration(values["for"])
			if err != nil {
				return tab.Say(tab.Status{Mark: page.Absent, Text: "nothing held"})
			}
			lid, _ := strconv.ParseBool(values["lid"])
			battery, _ := strconv.ParseBool(values["battery"])
			by := "station power awake"
			if lid {
				by += " --lid"
			}
			if battery {
				by += " --battery"
			}
			return tab.Exec(ID, func() (tab.Status, error) {
				a, err := m.power.Awake(m.ctx, power.Request{Lid: lid, Battery: battery, For: term, By: by})
				if err != nil {
					return tab.Status{}, err
				}
				return tab.Outcome("awake", "until", catalog.StateOK, a.Until.In(m.power.Zone()).Format("15:04")), nil
			})
		})
		return m, tea.Batch(cmd, fill)
	case pagepower.KeySleep:
		run := tab.Exec(ID, func() (tab.Status, error) {
			s, err := m.power.Sleep(m.ctx)
			if err != nil {
				return tab.Status{}, err
			}
			if !s.Ended {
				return tab.Status{Mark: page.Absent, Text: "no Power session to end"}, nil
			}
			return tab.Outcome("ended", "the Power session", catalog.StateOK, ""), nil
		})
		var act tea.Cmd
		m.View, act = m.Act(nil, run)
		return m, tea.Batch(cmd, act)
	}
	return m, cmd
}
