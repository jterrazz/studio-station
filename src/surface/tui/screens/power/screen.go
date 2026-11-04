package powerscreen

import (
	"context"
	"strconv"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/jterrazz/studio-station/src/awake"
	awakecalls "github.com/jterrazz/studio-station/src/awake/calls"
	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/core/call"
	"github.com/jterrazz/studio-station/src/core/machine"
	"github.com/jterrazz/studio-station/src/page"
	pagepower "github.com/jterrazz/studio-station/src/pages/power"
	"github.com/jterrazz/studio-station/src/surface/tui/screens/tab"
)

const ID = "power"

type Power interface {
	// value: the tab's pages: this host's own read first, then the daemon's
	// subscription while it answers, so a guard's end redraws the tab at once.
	Pages(ctx context.Context) <-chan page.Page
	Zone() *time.Location
}

type Deps struct {
	Power  Power
	Calls  call.Router
	Target machine.Ref
}

type Model struct {
	tab.View[page.Page]
	ctx  context.Context
	d    Deps
	here page.Context
}

func New(ctx context.Context, d Deps, here page.Context, describe tab.Describe) Model {
	source := func(ctx context.Context, _ bool) (<-chan page.Page, error) { return d.Power.Pages(ctx), nil }
	build := func(pg page.Page) page.Page { return pg }
	return Model{View: tab.NewView(ID, "Power", "power", here, source, build, describe), ctx: ctx, d: d, here: here}
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
			by := "station awake"
			if lid {
				by += " --lid"
			}
			if battery {
				by += " --battery"
			}
			return tab.Exec(ID, func() (tab.Status, error) {
				a, err := call.Invoke(m.ctx, m.d.Calls, m.d.Target, awakecalls.Keep, awake.Request{Lid: lid, Battery: battery, For: term, By: by}, nil)
				if err != nil {
					return tab.Status{}, err
				}
				return tab.Outcome("awake", "until", catalog.StateOK, a.Until.In(m.d.Power.Zone()).Format("15:04")), nil
			})
		})
		return m, tea.Batch(cmd, fill)
	case pagepower.KeySleep:
		run := tab.Exec(ID, func() (tab.Status, error) {
			s, err := call.Invoke(m.ctx, m.d.Calls, m.d.Target, awakecalls.Off, awakecalls.OffIn{}, nil)
			if err != nil {
				return tab.Status{}, err
			}
			if !s.Ended {
				return tab.Status{Mark: page.Absent, Text: "no Power session to end"}, nil
			}
			return tab.Outcome("ended", "the Power session", catalog.StateOK, ""), nil
		})
		var act tea.Cmd
		m.View, act = m.Act(run)
		return m, tea.Batch(cmd, act)
	}
	return m, cmd
}
