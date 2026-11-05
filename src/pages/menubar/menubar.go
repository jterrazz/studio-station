package menubar

import (
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/jterrazz/studio-station/src/activity"
	"github.com/jterrazz/studio-station/src/domain/machines"
	"github.com/jterrazz/studio-station/src/domain/power"
	"github.com/jterrazz/studio-station/src/domain/services"
	"github.com/jterrazz/studio-station/src/engine"
	"github.com/jterrazz/studio-station/src/page"
	pageengine "github.com/jterrazz/studio-station/src/pages/engine"
	pagefleet "github.com/jterrazz/studio-station/src/pages/fleet"
	pagehome "github.com/jterrazz/studio-station/src/pages/home"
	pagemachines "github.com/jterrazz/studio-station/src/pages/machines"
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
	ActionUpdateAll   = "update-all"
)

type Reading struct {
	// value: a nil part is still being read and draws pending.
	Home     *pagehome.Overview
	Samples  []activity.Reading
	Fleet    *machines.Fleet
	Services *services.Fleet
	Power    *power.Report
}

func Page(r Reading, here page.Context) page.Page {
	p := page.Page{ID: ID, Command: "menubar", Context: here, Strip: strip(r)}
	todo := todo(r, here)
	p.Sections = []page.Section{todo, fleet(r), powerSection(r)}
	p.Actions = append([]page.Action{newVM(), openStation(), stopAll(r)}, powerAction(r.Power))
	if updates(todo) {
		p.Actions = append(p.Actions, updateAll())
	}
	p.Badge = badge(r, todo)
	return page.Seal(p)
}

// invariant: the icon aggregates and never judges: the count is the to do's
// own, and a server is down by the machines model's rule; nil while the to
// do is still read.
func badge(r Reading, todo page.Section) *page.Badge {
	if r.Home == nil {
		return nil
	}
	b := page.Badge{}
	for _, t := range todo.Counts {
		if t.Mark != page.OK && t.Mark != page.Awaiting {
			b.Needs += t.N
		}
	}
	if r.Fleet != nil {
		for _, h := range r.Fleet.Hosts {
			b.Down = b.Down || slices.ContainsFunc(h.VMs, machines.Down)
		}
	}
	return &b
}

func Stopped(here page.Context, installed bool) page.Page {
	// invariant: the page the app draws when no daemon answers is bundled into
	// it, so it holds only what is true without one.
	row := page.Row{ID: "engine", Label: "engine", Mark: page.Attention, Reason: page.Stopped, Cells: []page.Value{page.Text("stopped")}}
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
		return page.Section{ID: "fleet", Title: "All machines", Columns: pagemachines.Columns(),
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

// value: the calls the menu bar's actions make, by their calls.json names.
const (
	CallCreate = "machines.create-workspace"
	CallStop   = "engine.stop"
	CallAwake  = "awake.keep"
	CallSleep  = "awake.off"
	CallUpdate = "apps.update"
)

// value: the command a to do's fix names when it is an update.
const updateFix = "station apps update"

func updates(todo page.Section) bool {
	for _, it := range todo.Items {
		if it.Row != nil && it.Row.Hint != nil && strings.HasPrefix(it.Row.Hint.Command, updateFix) {
			return true
		}
	}
	return false
}

// invariant: MB-16: Update all is the one apps.update of every Update
// available row; what fails stays in the to do with its fix.
func updateAll() page.Action {
	return page.Action{ID: ActionUpdateAll, Label: "Update all", On: page.OnPage, Priority: 2, Mutates: true, Run: &page.Run{Call: CallUpdate}}
}

func gigabytes(n int64) string { return strconv.FormatInt(n<<30, 10) }

func newVM() page.Action {
	return page.Action{ID: ActionNewVM, Label: "New VM", On: page.OnPage, Priority: 1, Mutates: true,
		Run: &page.Run{Call: CallCreate, In: map[string]string{"Request.Image": "linux", "Request.Network": "nat"}},
		Form: &page.Form{Title: "New VM", Fields: []page.Field{
			{Key: "Request.Lifetime", Label: "Lifetime", Kind: page.FieldSelect, Default: "throwaway", Options: []page.Option{{Value: "throwaway"}, {Value: "permanent"}}},
			{Key: "Request.Resources.MemoryBytes", Label: "Memory", Kind: page.FieldSelect, Default: gigabytes(4), Options: []page.Option{{Value: gigabytes(4), Label: "4G"}, {Value: gigabytes(8), Label: "8G"}}},
		}}}
}

func openStation() page.Action {
	return page.Action{ID: ActionOpenStation, Label: "Open station", On: page.OnPage, Priority: 2, Run: &page.Run{Terminal: true}}
}

func start() page.Action {
	return page.Action{ID: ActionStart, Label: "Start the engine", On: page.OnPage, Priority: 1, Mutates: true, Run: &page.Run{Terminal: true, Argv: []string{"engine", "start"}}}
}

func stopAll(r Reading) page.Action {
	var w engine.Work
	if r.Fleet != nil {
		for _, h := range r.Fleet.Hosts {
			if !h.Here {
				continue
			}
			for _, vm := range h.VMs {
				switch {
				case vm.State != machines.Running:
				case vm.Lifetime == machines.AlwaysOn:
					w.Servers = append(w.Servers, vm.Name)
				default:
					w.VMs = append(w.VMs, engine.Stopped{Name: vm.Name, Lifetime: string(vm.Lifetime), Result: engine.ResultStopped})
				}
			}
		}
	}
	confirm := pageengine.StopConfirm(w, page.ThisMac)
	return page.Action{ID: ActionStopAll, Label: "Stop everything", On: page.OnPage, Priority: 3, Mutates: true, Confirm: &confirm, Run: &page.Run{Call: CallStop, In: map[string]string{"Confirm": "yes", "By": "the menu bar"}}}
}

// invariant: the Power switch is the session row's action, awake with its
// form while off and sleep while a session holds.
func powerAction(r *power.Report) page.Action {
	rows := []string{pagepower.RowAwake}
	if r != nil && r.Session != nil && r.Daemon == power.DaemonRunning {
		return page.Action{ID: ActionPowerSleep, Label: "sleep", On: page.OnRow, Rows: rows, Priority: 4, Mutates: true, Run: &page.Run{Call: CallSleep}}
	}
	terms := []struct {
		label string
		hours int
	}{{"1h", 1}, {"2h", 2}, {"4h", 4}, {"8h", 8}, {"12h", 12}, {"24h", 24}}
	opts := make([]page.Option, 0, len(terms))
	for _, t := range terms {
		opts = append(opts, page.Option{Value: strconv.FormatInt(int64(time.Duration(t.hours)*time.Hour), 10), Label: t.label})
	}
	return page.Action{ID: ActionPowerAwake, Label: "awake", On: page.OnRow, Rows: rows, Priority: 4, Mutates: true, Run: &page.Run{Call: CallAwake},
		Form: &page.Form{Title: "Awake", Fields: []page.Field{
			{Key: "For", Label: "For", Kind: page.FieldSelect, Default: strconv.FormatInt(int64(4*time.Hour), 10), Options: opts},
			{Key: "Lid", Label: "Lid", Kind: page.FieldSelect, Default: "Lid=false", Options: []page.Option{
				{Value: "Lid=false", Label: "open only"}, {Value: "Lid=true", Label: "closed"}, {Value: "Lid=true Battery=true", Label: "closed on battery"},
			}},
		}}}
}

func Opening(id page.ID, here page.Context) page.Page {
	// invariant: the frame of one of the menu bar's own pages, before its
	// first read.
	return page.Page{ID: id, Context: here, State: page.Loading}
}

func Unread(id page.ID, here page.Context, err error) page.Page {
	return page.Seal(page.Page{ID: id, Context: here, Error: &page.Error{Message: err.Error()}})
}
