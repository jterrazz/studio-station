package machines

import (
	"strconv"
	"strings"
	"time"

	"github.com/jterrazz/studio-station/src/domain/machines"
	"github.com/jterrazz/studio-station/src/page"
)

func listColumns() []page.Column {
	return []page.Column{
		{Key: "name", Label: "MACHINE", Kind: page.KindText, Keep: 100},
		{Key: "kind", Label: "KIND", Kind: page.KindText, Keep: 80},
		{Key: "mac", Kind: page.KindText, Show: page.PipeOnly},
		{Key: "state", Label: "STATE", Kind: page.KindList, Keep: 90},
		{Key: "cpu", Label: "CPU", Kind: page.KindText, Keep: 40},
		{Key: "memory", Label: "MEMORY", Kind: page.KindText, Keep: 30},
		{Key: "detail", Kind: page.KindList, Keep: 5},
	}
}

func List(f machines.Fleet, warnings []string, here page.Context) page.Page {
	hereVersion := ""
	for _, h := range f.Hosts {
		if h.Here {
			hereVersion = h.StationVersion
		}
	}
	findings := machines.Needs(f)
	items := make([]page.Item, 0, len(f.Hosts))
	var hints []page.Hint
	for _, h := range f.Hosts {
		g := page.Group{Head: macRow(machines.Label(f, h), h, hereVersion, f.Read), Fold: page.Open}
		for _, vm := range machines.Ordered(h.VMs) {
			row := listVMRow(h.Name, vm)
			g.Items = append(g.Items, page.Item{Row: &row})
		}
		if len(h.VMs) == 0 && h.Reachable {
			g.Items = append(g.Items, page.Item{Empty: &page.Empty{Noun: "VMs", Text: "No VMs yet"}})
			create := "station machines create"
			if !h.Here {
				create += " --on " + machines.Label(f, h)
			}
			hints = append(hints, page.Hint{Command: create})
		}
		items = append(items, page.Item{Group: &g})
	}
	if len(items) == 0 {
		items = append(items, page.Item{Empty: &page.Empty{Noun: "Macs", Hint: page.Cmd("station machines add <mac>")}})
	}
	title, caption := firstLine(machines.Tally(f), len(findings))
	sections := []page.Section{{ID: "machines", Title: title, Caption: caption, Joined: true, Role: page.Banner, Uncounted: true, Columns: listColumns(), Items: items}}
	if len(findings) > 0 {
		sections = append(sections, needsYou(findings))
	}
	if len(warnings) > 0 {
		notes := make([]page.Item, 0, len(warnings))
		for _, w := range warnings {
			notes = append(notes, page.Item{Note: &page.Note{Text: w}})
		}
		sections = append(sections, page.Section{ID: "warnings", Title: "Warnings", Role: page.Banner, Items: notes})
	}
	return page.Seal(page.Page{ID: "machines", Command: "machines", Context: here, Sections: sections, Hints: hints, Actions: Actions()})
}

func firstLine(c machines.Counts, needs int) (string, []page.Value) {
	parts := []page.Value{page.Text(strconv.Itoa(c.Running) + " running"), page.Text(strconv.Itoa(c.Stopped) + " stopped")}
	switch needs {
	case 0:
		parts = append(parts, page.Text("nothing needs you"))
	case 1:
		parts = append(parts, page.Text("1 needs you"))
	default:
		parts = append(parts, page.Text(strconv.Itoa(needs)+" need you"))
	}
	return many(c.Machines, "machine"), parts
}

func many(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	return strconv.Itoa(n) + " " + noun + "s"
}

func needsYou(fs []machines.Need) page.Section {
	items := make([]page.Item, 0, len(fs))
	for i, f := range fs {
		mark, reason := page.Attention, page.Degraded
		if f.Broken {
			mark, reason = page.Broken, page.Failing
		}
		says := strings.TrimPrefix(f.Text, f.Subject+" ")
		row := page.Row{ID: strconv.Itoa(i) + "-" + f.Subject, Label: f.Subject, Mark: mark, Reason: reason, Cells: []page.Value{page.Text(says), fixOf(f)}}
		items = append(items, page.Item{Row: &row})
	}
	columns := []page.Column{{Key: "subject", Kind: page.KindText, Keep: 100}, {Key: "finding", Kind: page.KindList, Keep: 90}, {Key: "fix", Kind: page.KindList, Keep: 50}}
	return page.Section{ID: "needs-you", Title: "Needs you", Uncounted: true, Columns: columns, Items: items}
}

func fixOf(f machines.Need) page.Value {
	switch {
	case f.Fix == "":
		return page.None()
	case strings.HasPrefix(f.Fix, "station "):
		return page.Command(f.Fix)
	}
	return page.Text(f.Fix)
}

func macRow(label string, h machines.HostView, hereVersion string, now time.Time) page.Row {
	row := page.Row{ID: h.Name, Label: h.Name, Short: label, Mark: page.OK}
	state, detail := page.Text("Online"), page.None()
	switch {
	case !h.Reachable && !h.Answered.IsZero():
		row.Mark, row.Reason, state = page.Broken, page.Unreachable, page.List(page.Text("Unreachable"), page.Text("last answer "+lastAnswer(h.Answered, now)))
	case !h.Reachable:
		row.Mark, row.Reason, state = page.Broken, page.Unreachable, page.Text("Unreachable")
	case h.Daemon.Known && !h.Daemon.Running && !h.Daemon.Installed && h.Here:
		row.Mark, row.Reason, state = page.Broken, page.Missing, page.Text("Engine not set up")
	case h.Daemon.Known && !h.Daemon.Running:
		row.Mark, row.Reason, state = page.Broken, page.Stopped, page.Text("Engine stopped")
	case h.Daemon.Known:
		state = page.Text("Engine running")
	}
	if _, ok := machines.Behind(h, hereVersion); ok && h.Reachable {
		row.Mark, row.Reason = page.Attention, page.Mismatch
		state = page.List(state, page.Text("runs "+h.StationVersion))
	}
	row.Cells = []page.Value{page.Text("Mac"), page.Text(h.Name), state, page.None(), page.None(), detail}
	return row
}

func listVMRow(mac string, vm machines.VM) page.Row {
	row := page.Row{ID: vm.Name, Label: vm.Name, Mark: page.OK}
	lifetime := page.Text(vm.Lifetime.Word())
	state, detail := page.List(page.Text("Running"), lifetime), page.None()
	switch vm.State {
	case machines.Running:
	case machines.Stopped:
		row.Mark, state = page.Absent, page.List(page.Text("Stopped"), lifetime)
		if vm.Detail != "" {
			detail = page.Text(vm.Detail)
		}
	case machines.Suspended:
		row.Mark, row.Reason, state = page.Attention, page.Stopped, page.List(page.Text("Suspended"), lifetime)
	case machines.Stale:
		row.Mark, row.Reason, state = page.Attention, page.Outlived, page.List(page.Text("Ended unexpectedly"), lifetime)
	case machines.Broken:
		row.Mark, row.Reason, state = page.Broken, page.Failing, page.List(page.Text("In a loop"), lifetime)
		detail = page.List(page.Text(vm.Detail), page.Text("stopped"))
	case machines.Unknown:
		row.Mark, row.Reason, state = page.NotKnown, page.Unreachable, page.List(page.Text("Unknown"), lifetime)
	}
	r := vm.Resources
	row.Cells = []page.Value{page.Text(machines.KindWord(vm.Lifetime.Kind())), page.Text(mac), state, page.Text(strconv.Itoa(r.CPUs) + " CPU"), Memory(r.MemoryBytes), detail}
	return row
}

// value: the hour of a Mac's last answer, with its day when that was not
// today.
func lastAnswer(at, now time.Time) string {
	at, now = at.Local(), now.Local()
	if at.YearDay() == now.YearDay() && at.Year() == now.Year() {
		return at.Format("15:04")
	}
	return at.Format("2 Jan 15:04")
}
