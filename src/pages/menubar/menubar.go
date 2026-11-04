package menubar

import (
	"strconv"

	"github.com/jterrazz/studio-station/src/activity"
	"github.com/jterrazz/studio-station/src/domain/machines"
	"github.com/jterrazz/studio-station/src/domain/power"
	"github.com/jterrazz/studio-station/src/domain/services"
	"github.com/jterrazz/studio-station/src/engine"
	"github.com/jterrazz/studio-station/src/page"
	pagefleet "github.com/jterrazz/studio-station/src/pages/fleet"
	pagehome "github.com/jterrazz/studio-station/src/pages/home"
	pagemachines "github.com/jterrazz/studio-station/src/pages/machines"
	pageoutcome "github.com/jterrazz/studio-station/src/pages/outcome"
	pagepower "github.com/jterrazz/studio-station/src/pages/power"
)

const ID page.ID = "menubar"

const (
	shown = 3

	ActionNewVM       = "new-vm"
	ActionOpenStation = "open-station"
	ActionStopAll     = "stop-all"
	ActionStart       = "start"
	ActionPowerAwake  = "power-awake"
	ActionPowerSleep  = "power-sleep"
)

type Reading struct {
	// value: a nil part is still being read and draws pending.
	Home     *pagehome.Overview
	Samples  []activity.Sample
	Fleet    *machines.Fleet
	Services *services.Fleet
	Power    *power.Report
}

func Page(r Reading, here page.Context) page.Page {
	p := page.Page{ID: ID, Command: "menubar", Context: here, Strip: strip(r)}
	p.Sections = []page.Section{todo(r, here), fleet(r), powerSection(r)}
	p.Actions = append([]page.Action{newVM(), openStation(), stopAll(r)}, powerAction(r.Power))
	return page.Seal(p)
}

func Stopped(here page.Context, installed bool) page.Page {
	// invariant: the page the app draws when no daemon answers is bundled into
	// it, so it holds only what is true without one.
	row := page.Row{ID: "daemon", Label: "daemon", Mark: page.Attention, Reason: page.Stopped, Cells: []page.Value{page.Text("stopped")}}
	actions := []page.Action{start(), openStation()}
	if !installed {
		row.Cells = []page.Value{page.Text("not installed")}
		row.Hint = page.Cmd("station setup")
		actions = actions[1:]
	}
	s := page.Section{ID: "todo", Title: "To do", Role: page.Actions, Columns: columns(), Items: []page.Item{{Row: &row}}}
	return page.Seal(page.Page{ID: ID, Command: "menubar", Context: here, Sections: []page.Section{s}, Actions: actions})
}

func columns() []page.Column {
	return []page.Column{{Key: "name", Kind: page.KindText, Keep: 100}, {Key: "value", Kind: page.KindList, Keep: 60}}
}

func strip(r Reading) *page.Strip {
	if len(r.Samples) == 0 || r.Home == nil {
		return &page.Strip{CPU: page.Pending(), Memory: page.Pending()}
	}
	s := pagehome.Strip(r.Samples, *r.Home)
	s.GPU, s.Net = page.Value{}, nil
	if r.Power != nil && r.Power.Reading.HasBattery {
		s.Battery = page.Percent(int64(r.Power.Reading.Percent) * 10)
	}
	return &s
}

// invariant: the badge counts every action with its worst mark while only the
// first three are drawn, so the section keeps the full counts (Uncounted)
// and the rest is one link to Home.
func todo(r Reading, here page.Context) page.Section {
	s := page.Section{ID: "todo", Title: "To do", Role: page.Actions, Uncounted: true, Columns: columns()}
	if r.Home == nil {
		s.Uncounted = false
		s.Items = []page.Item{{Row: &page.Row{ID: "reading", Label: "reading", Mark: page.Awaiting}}}
		return s
	}
	home := pagehome.Snapshot(*r.Home, here, false).Sections[0]
	var rows []page.Item
	counts := map[page.Mark]int{}
	for _, it := range home.Items {
		if it.Row == nil {
			continue
		}
		counts[it.Row.Mark]++
		rows = append(rows, it)
	}
	for _, m := range page.Marks() {
		if counts[m] > 0 {
			s.Counts = append(s.Counts, page.Tally{Mark: m, N: counts[m]})
		}
	}
	switch {
	case len(rows) == 0:
		s.Items = []page.Item{{Empty: &page.Empty{Noun: "actions"}}}
	case len(rows) > shown:
		more := page.Link{Label: "+ " + strconv.Itoa(len(rows)-shown) + " more", Page: "home"}
		rows = append(rows[:shown:shown], page.Item{Link: &more})
		s.Items = rows
	default:
		s.Items = rows
	}
	return s
}

func fleet(r Reading) page.Section {
	if r.Fleet == nil {
		return page.Section{ID: "fleet", Title: "Fleet", Columns: pagemachines.Columns(),
			Items: []page.Item{{Row: &page.Row{ID: "fleet", Label: "reading", Mark: page.Awaiting}}}}
	}
	return pagefleet.Machines(*r.Fleet, r.Services)
}

func powerSection(r Reading) page.Section {
	if r.Power == nil {
		return pagepower.Session(power.Report{Daemon: power.DaemonPending})
	}
	return pagepower.Session(*r.Power)
}

func newVM() page.Action {
	return page.Action{ID: ActionNewVM, Label: "New VM", On: page.OnPage, Priority: 1, Mutates: true,
		Run: &page.Run{Argv: []string{"machines", "create"}},
		Form: &page.Form{Title: "New VM", Fields: []page.Field{
			{Key: "lifetime", Label: "Lifetime", Kind: page.FieldSelect, Default: "temporary", Options: []page.Option{{Value: "temporary"}, {Value: "lasting"}}},
			{Key: "memory", Label: "Memory", Kind: page.FieldSelect, Default: "4G", Options: []page.Option{{Value: "4G"}, {Value: "8G"}}},
		}}}
}

func openStation() page.Action {
	return page.Action{ID: ActionOpenStation, Label: "Open station", On: page.OnPage, Priority: 2, Run: &page.Run{Terminal: true}}
}

func start() page.Action {
	return page.Action{ID: ActionStart, Label: "Start station", On: page.OnPage, Priority: 1, Mutates: true, Run: &page.Run{Argv: []string{"start"}}}
}

func stopAll(r Reading) page.Action {
	var w engine.Work
	host := "this host"
	if r.Fleet != nil {
		for _, h := range r.Fleet.Hosts {
			if !h.Here {
				continue
			}
			host = h.Name
			for _, vm := range h.VMs {
				switch {
				case vm.Lifetime == machines.Temporary:
					w.VMs = append(w.VMs, engine.Stopped{Name: vm.Name, Lifetime: string(vm.Lifetime), Result: engine.ResultDeleted})
				case vm.State == machines.Running:
					w.VMs = append(w.VMs, engine.Stopped{Name: vm.Name, Lifetime: string(vm.Lifetime), Result: engine.ResultStopped})
				}
			}
		}
	}
	confirm := pageoutcome.StopConfirm(w, host)
	return page.Action{ID: ActionStopAll, Label: "Stop everything", On: page.OnPage, Priority: 3, Mutates: true, Confirm: &confirm, Run: &page.Run{Argv: []string{"stop", "--yes"}}}
}

// invariant: the Power switch is the session row's action, awake with its
// form while off and sleep while a session holds.
func powerAction(r *power.Report) page.Action {
	rows := []string{pagepower.RowAwake}
	if r != nil && r.Session != nil && r.Daemon == power.DaemonRunning {
		return page.Action{ID: ActionPowerSleep, Label: "sleep", On: page.OnRow, Rows: rows, Priority: 4, Mutates: true, Run: &page.Run{Op: "power_sleep"}}
	}
	terms := []struct {
		label string
		hours int
	}{{"1h", 1}, {"2h", 2}, {"4h", 4}, {"8h", 8}, {"12h", 12}, {"24h", 24}}
	opts := make([]page.Option, 0, len(terms))
	for _, t := range terms {
		opts = append(opts, page.Option{Value: strconv.Itoa(t.hours * 3600), Label: t.label})
	}
	return page.Action{ID: ActionPowerAwake, Label: "awake", On: page.OnRow, Rows: rows, Priority: 4, Mutates: true, Run: &page.Run{Op: "power_awake"},
		Form: &page.Form{Title: "Power", Fields: []page.Field{
			{Key: "for", Label: "For", Kind: page.FieldSelect, Default: "14400", Options: opts},
			{Key: "lid", Label: "Lid", Kind: page.FieldSelect, Default: "lid=false", Options: []page.Option{
				{Value: "lid=false", Label: "open only"}, {Value: "lid=true", Label: "closed"}, {Value: "lid=true battery=true", Label: "closed on battery"},
			}},
		}}}
}
