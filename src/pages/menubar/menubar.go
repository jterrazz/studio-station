package menubar

import (
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/jterrazz/studio-station/src/activity"
	"github.com/jterrazz/studio-station/src/awake"
	"github.com/jterrazz/studio-station/src/core/machine"

	"github.com/jterrazz/studio-station/src/engine"
	model "github.com/jterrazz/studio-station/src/machines"
	"github.com/jterrazz/studio-station/src/overview"
	"github.com/jterrazz/studio-station/src/page"
	pageengine "github.com/jterrazz/studio-station/src/pages/engine"
	pagehome "github.com/jterrazz/studio-station/src/pages/home"
	pagekeepawake "github.com/jterrazz/studio-station/src/pages/keepawake"
	"github.com/jterrazz/studio-station/src/ports"
)

const ID page.ID = "menubar"

const (
	shown = 3

	ActionNewVM       = "new-vm"
	ActionOpenStation = "open-station"
	ActionStopAll     = "stop-all"
	ActionStart       = "start"
	ActionKeepAwake   = "awake-keep"
	ActionAwakeOff    = "awake-off"
	ActionUpdateAll   = "update-all"

	SectionNeeds    = "needs-you"
	SectionMachines = "machines"
	SectionAwake    = "awake"
	SectionAccounts = "accounts"
)

type Reading struct {
	// value: a nil part is still being read and draws pending.
	Home    *overview.Summary
	Samples []activity.Reading
	All     *model.AllMachines
	Ports   *ports.AllMachines
	Awake   *awake.Report
	// value: every other Mac's Awake, as its engine reports it.
	Macs []awake.Report
}

func Page(r Reading, thisMac page.Context) page.Page {
	p := page.Page{ID: ID, Command: "menubar", Context: thisMac, Strip: strip(r.Samples)}
	needs := needsYou(r)
	p.Sections = []page.Section{needs, Tree(r.All, r.Ports, homeLine(r)), awakeTile(r), accountsTile(r.Home)}
	p.Actions = append([]page.Action{newVM(), openStation(), stopAll(r)}, awakeAction(r.Awake))
	if update := updateAll(needs, r.Home); update != nil {
		p.Actions = append(p.Actions, *update)
	}
	p.Badge = badge(r, needs)
	return page.Seal(p)
}

// invariant: the icon aggregates and never judges: the count is Needs you's
// own, and a server is down by the machines model's rule; nil while Needs
// you is still read.
func badge(r Reading, needs page.Section) *page.Badge {
	if r.Home == nil {
		return nil
	}
	b := page.Badge{}
	for _, t := range needs.Counts {
		if t.Mark != page.OK && t.Mark != page.Awaiting {
			b.Needs += t.N
		}
	}
	if r.All != nil {
		for _, h := range r.All.Macs {
			b.Down = b.Down || slices.ContainsFunc(h.VMs, model.Down)
		}
	}
	return &b
}

func Stopped(thisMac page.Context, installed bool) page.Page {
	// invariant: the page the app draws when no daemon answers is bundled into
	// it, so it holds only what is true without one.
	row := page.Row{ID: "engine", Label: "engine", Mark: page.Attention, Reason: page.Stopped, Cells: []page.Value{page.Text("stopped")}}
	actions := []page.Action{start(), openStation()}
	if !installed {
		row.Cells = []page.Value{page.Text("not installed")}
		row.Hint = page.Cmd("station setup")
		actions = actions[1:]
	}
	s := page.Section{ID: SectionNeeds, Title: "Needs you", Role: page.Actions, Columns: columns(), Items: []page.Item{{Row: &row}}}
	return page.Seal(page.Page{ID: ID, Command: "menubar", Context: thisMac, Sections: []page.Section{s}, Actions: actions})
}

func columns() []page.Column {
	return []page.Column{{Key: "name", Kind: page.KindText, Keep: 100}, {Key: "value", Kind: page.KindList, Keep: 60}}
}

// value: the three measures of This Mac, CPU, Memory and Network, each with
// its sparkline over the readings kept.
func strip(readings []activity.Reading) *page.Strip {
	if len(readings) < 2 {
		return &page.Strip{CPU: page.Pending(), Memory: page.Pending()}
	}
	var s page.Strip
	for i := 1; i < len(readings); i++ {
		rates := activity.Between(readings[i-1].Sample, readings[i].Sample)
		s.Spark = append(s.Spark, int(rates.CPU))
		s.NetSpark = append(s.NetSpark, int((rates.NetIn+rates.NetOut)/1e3))
		if m := readings[i].Sample; m.MemTotal > 0 {
			s.MemorySpark = append(s.MemorySpark, int(m.MemUsed*100/m.MemTotal))
		}
	}
	last := readings[len(readings)-1].Sample
	rates := activity.Between(readings[len(readings)-2].Sample, last)
	s.CPU = page.Percent(int64(rates.CPU * 10))
	if last.MemTotal > 0 {
		s.Memory = page.Ratio(min(last.MemUsed, last.MemTotal), last.MemTotal)
	}
	net := page.Size(int64(rates.NetIn + rates.NetOut))
	net.Unit = "/s"
	s.Net = []page.Value{net}
	return &s
}

// invariant: the badge counts every finding while only the first three are
// drawn, so the section keeps the full counts (Uncounted) and its caption
// says them all; the rest is one link to the Overview.
func needsYou(r Reading) page.Section {
	s := page.Section{ID: SectionNeeds, Title: "Needs you", Role: page.Actions, Uncounted: true, Columns: columns()}
	if r.Home == nil {
		s.Uncounted = false
		s.Items = []page.Item{{Row: &page.Row{ID: "reading", Label: "reading", Mark: page.Awaiting}}}
		return s
	}
	var rows []page.Item
	counts := map[page.Mark]int{}
	for _, body := range pagehome.Body(*r.Home, true) {
		if body.ID != SectionNeeds {
			continue
		}
		for _, it := range body.Items {
			if it.Row == nil {
				continue
			}
			counts[it.Row.Mark]++
			rows = append(rows, it)
		}
	}
	for _, m := range page.Marks() {
		if counts[m] > 0 {
			s.Counts = append(s.Counts, page.Tally{Mark: m, N: counts[m]})
		}
	}
	s.Caption = []page.Value{page.Text(things(len(rows)))}
	switch {
	case len(rows) == 0:
		s.Items = []page.Item{{Empty: &page.Empty{Text: "Nothing needs you"}}}
	case len(rows) > shown:
		more := page.Link{Label: "+ " + strconv.Itoa(len(rows)-shown) + " more", Page: "overview"}
		rows = append(rows[:shown:shown], page.Item{Link: &more})
		s.Items = rows
	default:
		s.Items = rows
	}
	return s
}

func things(n int) string {
	switch n {
	case 0:
		return "nothing needs you"
	case 1:
		return "1 thing needs you"
	}
	return strconv.Itoa(n) + " things need you"
}

// invariant: MB-16: a Needs you line whose fix is the apps' update gets
// Update all, which runs that call on This Mac with no name: every update
// there; what fails stays in Needs you with its fix.
func updateAll(needs page.Section, home *overview.Summary) *page.Action {
	if home == nil {
		return nil
	}
	var rows []string
	for i, n := range home.Needs {
		id := strconv.Itoa(i) + "-" + n.Subject
		if n.Call == CallUpdate && slices.ContainsFunc(needs.Items, func(it page.Item) bool { return it.Row != nil && it.Row.ID == id }) {
			rows = append(rows, id)
		}
	}
	if len(rows) == 0 {
		return nil
	}
	return &page.Action{ID: ActionUpdateAll, Label: "Update all", On: page.OnRow, Rows: rows, Priority: 5, Mutates: true, Run: &page.Run{Call: CallUpdate}}
}

// value: the line under a Mac in Home: its Awake.
func homeLine(r Reading) func(h model.MacView) page.Value {
	return func(h model.MacView) page.Value {
		if h.ThisMac {
			if r.Awake != nil {
				if w := awakeWords(*r.Awake); w != "" {
					return page.Text("awake " + w)
				}
			}
			return page.None()
		}
		for _, m := range r.Macs {
			if m.Mac == h.Name && m.NeverSleeps {
				return page.Text("never sleeps")
			}
		}
		return page.None()
	}
}

func awakeWords(r awake.Report) string {
	switch {
	case r.Session != nil:
		zone := r.Zone
		if zone == nil {
			zone = time.UTC
		}
		return "until " + r.Session.Until.In(zone).Format("15:04")
	case r.NeverSleeps:
		return "never sleeps"
	}
	return ""
}

// value: the Awake tile: This Mac's keep-awake first, then each other Mac's
// mode; its switch is This Mac's.
func awakeTile(r Reading) page.Section {
	var parts []page.Value
	self := ""
	if r.Awake != nil {
		self = r.Awake.Mac
		parts = append(parts, page.Text(page.ThisMac+" "+cmpWord(awakeWords(*r.Awake), "sleeps normally")))
	}
	for _, m := range r.Macs {
		if !m.ThisMac {
			parts = append(parts, page.Text(page.MacName(m.Mac, self)+" "+cmpWord(awakeWords(m), "sleeps normally")))
		}
	}
	value := page.Pending()
	if r.Awake != nil {
		value = page.List(parts...)
	}
	row := page.Row{ID: pagekeepawake.RowAwake, Label: "Awake", Cells: []page.Value{value}}
	return page.Section{ID: SectionAwake, Title: "Awake", Role: page.Summary, Columns: columns(), Items: []page.Item{{Row: &row}}}
}

func cmpWord(s, otherwise string) string {
	if s == "" {
		return otherwise
	}
	return s
}

// value: the Accounts tile: This Mac's Accounts line of its Overview.
func accountsTile(home *overview.Summary) page.Section {
	row := page.Row{ID: "accounts", Label: "Accounts", Cells: []page.Value{page.Pending()}}
	if home != nil {
		for _, body := range pagehome.Body(*home, true) {
			for _, it := range body.Items {
				if body.ID == "sections" && it.Row != nil && it.Row.ID == "accounts" {
					row.Cells = it.Row.Cells
				}
			}
		}
	}
	return page.Section{ID: SectionAccounts, Title: "Accounts", Role: page.Summary, Columns: columns(), Items: []page.Item{{Row: &row}}}
}

// value: the calls the menu bar's actions make, by their calls.json names.
const (
	CallCreate    = "machines.create-workspace"
	CallStop      = "engine.stop"
	CallKeepAwake = "awake.keep"
	CallAwakeOff  = "awake.off"
	CallUpdate    = "apps.update"
	CallAddMac    = "machines.add"
	CallStopVM    = "machines.stop"
	CallDeleteVM  = "machines.delete"
	CallRestartVM = "machines.restart"
	CallSharePort = "ports.share"
	CallTakeBack  = "accounts.take-back"
)

func gigabytes(n int64) string { return strconv.FormatInt(n<<30, 10) }

// value: New VM…'s sheet: the create form of the CLI and the TUI, its
// questions in the same words.
func newVM() page.Action {
	return page.Action{ID: ActionNewVM, Key: "cmd+n", Label: "New VM", On: page.OnPage, Priority: 1, Mutates: true,
		Run: &page.Run{Call: CallCreate, In: map[string]string{"Request.Image": "linux", "Request.Network": "nat"}},
		Form: &page.Form{Title: "New VM", Fields: []page.Field{
			{Key: "Request.Lifetime", Label: "Will it last?", Kind: page.FieldSelect, Default: "throwaway", Options: []page.Option{
				{Value: "throwaway", Label: "Throwaway", Note: "deleted when it powers off from inside"},
				{Value: "permanent", Label: "Permanent", Note: "kept until you delete it"},
			}},
			{Key: "Request.Resources.MemoryBytes", Label: "Memory", Kind: page.FieldSelect, Default: gigabytes(4), Options: []page.Option{{Value: gigabytes(4), Label: "4 GB"}, {Value: gigabytes(8), Label: "8 GB"}}},
		}}}
}

// value: Open in Terminal: a new terminal runs station, on the machine
// these words name; the one way the menu bar starts a program.
func openIn(words ...string) page.Action {
	return page.Action{ID: ActionOpenStation, Key: "cmd+t", Label: "Open in Terminal", On: page.OnPage, Priority: 2, Run: &page.Run{Terminal: true, Argv: words}}
}

func openStation() page.Action { return openIn() }

func start() page.Action {
	return page.Action{ID: ActionStart, Label: "Start the engine", On: page.OnPage, Priority: 1, Mutates: true, Run: &page.Run{Terminal: true, Argv: []string{"engine", "start"}}}
}

func stopAll(r Reading) page.Action {
	var w engine.Work
	if r.All != nil {
		for _, h := range r.All.Macs {
			if !h.ThisMac {
				continue
			}
			for _, vm := range h.VMs {
				switch {
				case vm.State != model.Running:
				case vm.Lifetime == model.AlwaysOn:
					w.Servers = append(w.Servers, vm.Name)
				default:
					w.VMs = append(w.VMs, engine.Stopped{Name: vm.Name, Lifetime: string(vm.Lifetime), Result: engine.ResultStopped})
				}
			}
		}
	}
	c := pageengine.StopConfirm(w, page.ThisMac)
	return page.Action{ID: ActionStopAll, Label: "Stop everything on " + page.ThisMac, On: page.OnPage, Priority: 3, Mutates: true, Confirm: &c,
		Run: &page.Run{Call: CallStop, In: map[string]string{"Confirm": yes, "By": "the menu bar"}}}
}

// invariant: the Awake switch is the tile's action, awake with its form
// while off and sleep while a session holds.
func awakeAction(r *awake.Report) page.Action {
	rows := []string{pagekeepawake.RowAwake}
	if r != nil && r.Session != nil && r.Engine == awake.EngineRunning {
		return page.Action{ID: ActionAwakeOff, Label: "sleep", On: page.OnRow, Rows: rows, Priority: 4, Mutates: true, Run: &page.Run{Call: CallAwakeOff}}
	}
	terms := []int{1, 4, 8}
	opts := make([]page.Option, 0, len(terms))
	for _, h := range terms {
		opts = append(opts, page.Option{Value: strconv.FormatInt(int64(time.Duration(h)*time.Hour), 10), Label: strconv.Itoa(h) + " h"})
	}
	return page.Action{ID: ActionKeepAwake, Label: "awake", On: page.OnRow, Rows: rows, Priority: 4, Mutates: true, Run: &page.Run{Call: CallKeepAwake},
		Form: &page.Form{Title: "Awake", Fields: []page.Field{
			{Key: "For", Label: "Keep awake for", Kind: page.FieldSelect, Default: strconv.FormatInt(int64(4*time.Hour), 10), Options: opts},
			{Key: "Lid", Label: "Lid", Kind: page.FieldSelect, Default: "Lid=false", Options: []page.Option{
				{Value: "Lid=false", Label: "open only"}, {Value: "Lid=true", Label: "closed"}, {Value: "Lid=true Battery=true", Label: "closed on battery"},
			}},
		}}}
}

type Levels func(gesture string, kind machine.Kind) string

// value: the levels a gesture on a machine of a kind asks at, as the
// call's confirmation rule gives them (confirm.For, its caller's): a page
// only draws them.
const (
	LevelNone     = "none"
	LevelYesNo    = "yes-no"
	LevelTypeName = "type-name"
	yes           = "yes"
)

// value: the question a gesture on a machine asks, at the level the call
// asks it, never the surface's: none, yes or no, or the name to type.
func ask(levels Levels, gesture string, k machine.Kind, name, question, verb string, lines ...string) *page.Confirm {
	values := make([]page.Value, 0, len(lines)+1)
	for _, l := range lines {
		values = append(values, page.Text(l))
	}
	switch levels(gesture, k) {
	case LevelNone:
		return nil
	case LevelTypeName:
		values = append(values, page.Text("Type "+name+" to "+verb+" it"))
		return &page.Confirm{Question: question, Lines: values, Danger: true, Type: name, Yes: titled(verb)}
	}
	return &page.Confirm{Question: question, Lines: values, Yes: titled(verb)}
}

// value: the call's input that answers the question ask drew: yes for a
// yes or no, nothing for a typed one, whose typed name the sheet adds.
func answered(c *page.Confirm, in map[string]string) map[string]string {
	if c != nil && c.Type == "" {
		in["Confirm"] = yes
	}
	return in
}

func titled(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func Opening(id page.ID, thisMac page.Context) page.Page {
	// invariant: the frame of one of the menu bar's own pages, before its
	// first read.
	return page.Page{ID: id, Context: thisMac, State: page.Loading}
}

func Unread(id page.ID, thisMac page.Context, err error) page.Page {
	return page.Seal(page.Page{ID: id, Context: thisMac, Error: &page.Error{Message: err.Error()}})
}
