package activity

import (
	"strconv"
	"strings"
	"time"

	"github.com/jterrazz/studio-station/src/activity"
	"github.com/jterrazz/studio-station/src/page"
)

func fleetColumns() []page.Column {
	return []page.Column{
		{Key: "machine", Kind: page.KindText, Keep: 100},
		{Key: "kind", Label: "KIND", Kind: page.KindText, Keep: 90},
		{Key: "cpu", Label: "CPU", Kind: page.KindText, Keep: 90},
		{Key: "memory", Label: "MEMORY", Kind: page.KindText, Keep: 80},
		{Key: "disk", Label: "DISK R/W", Kind: page.KindText, Keep: 60},
		{Key: "net", Label: "NET IN/OUT", Kind: page.KindText, Keep: 60},
		{Key: "free", Label: "FREE", Kind: page.KindText, Keep: 70},
	}
}

func pair(a, b float64) string {
	mb := func(x float64) string { return strconv.FormatFloat(x/1e6, 'f', 1, 64) }
	return mb(a) + "/" + mb(b) + " MB/s"
}

func fleetRow(v View) page.Row {
	// invariant: one row per machine (SPEC §6.4): a stopped VM says so, a
	// Mac that does not answer is its own row and the others are filled.
	r := page.Row{ID: v.Name, Label: v.Label, Mark: page.OK, Cells: []page.Value{page.Text(kindWord(v.Kind))}}
	rep := v.Report
	switch {
	case v.Silent != "":
		r.Mark, r.Reason = page.Broken, page.Unreachable
		r.Cells = append(r.Cells, page.Text(v.Silent))
		return r
	case v.Stopped:
		r.Mark, r.Reason = page.Absent, page.Stopped
		r.Cells = append(r.Cells, page.Text("Stopped"))
		return r
	case rep.Silent:
		r.Mark, r.Reason = page.Broken, page.Late
		r.Cells = append(r.Cells, silent())
		return r
	case rep.Last.Run != nil:
		r.Cells = append(r.Cells, page.Text(activity.Percent(rep.Last.Run.CPU)+" of "+v.On), page.Text(activity.Memory(rep.Last.Run.Memory)+" resident"))
		return r
	}
	s := rep.Last.Sample
	cpu, disk, net, memory := "?", "?", "?", "?"
	if rep.Window > 0 && counted(s) {
		cpu = activity.Percent(rep.Rates.CPU)
		disk, net = pair(rep.Rates.DiskRead, rep.Rates.DiskWrite), pair(rep.Rates.NetIn, rep.Rates.NetOut)
	}
	free := page.None()
	if s.DiskTotal > 0 {
		free = page.Text(activity.Bytes(s.DiskFree))
	}
	if s.MemTotal > 0 {
		memory = strings.TrimSuffix(activity.Memory(s.MemUsed), " GB") + "/" + activity.Memory(s.MemTotal)
	}
	r.Cells = append(r.Cells, page.Text(cpu), page.Text(memory), page.Text(disk), page.Text(net), free)
	return r
}

func Fleet(views []View, here page.Context) page.Page {
	p := page.Page{ID: "activity-all", Command: "activity --all-machines", Context: here}
	answering := 0
	var kept, direct time.Duration
	for _, v := range views {
		if v.Silent != "" || v.Stopped || v.Report.Silent {
			continue
		}
		answering++
		if v.Report.Source == activity.FromDirect {
			direct = max(direct, v.Report.Window)
		} else {
			kept = max(kept, v.Report.Window)
		}
	}
	words := []string{strconv.Itoa(len(views)) + " machines", strconv.Itoa(answering) + " answering"}
	switch {
	case kept > 0:
		words = append(words, "rates over the last "+activity.Window(kept))
	case direct > 0:
		words = append(words, "rates over "+activity.Window(direct))
	}
	items := make([]page.Item, 0, len(views))
	needs := make([]page.Item, 0, len(views))
	for _, v := range views {
		r := fleetRow(v)
		items = append(items, page.Item{Row: &r})
		needs = append(needs, Needs(v)...)
	}
	for i := range needs {
		needs[i].Row.ID = "finding-" + strconv.Itoa(i)
	}
	banner := page.Row{ID: "fleet", Label: "All machines", Mark: page.OK, Cells: []page.Value{page.Texts(words...)}}
	p.Sections = append(p.Sections, page.Section{ID: "fleet", Title: "Activity", Role: page.Banner, Uncounted: true, Columns: columns(), Items: []page.Item{{Row: &banner}}})
	if len(needs) > 0 {
		p.Sections = append(p.Sections, page.Section{ID: "needs", Title: "Needs you", Uncounted: true, Columns: columns()[:1], Items: needs})
	}
	p.Sections = append(p.Sections, page.Section{ID: "machines", Title: "Machines", Uncounted: true, Columns: fleetColumns(), Items: items})
	return page.Seal(p)
}

func FleetContract(views []View, now time.Time) FleetJSON {
	out := FleetJSON{Schema: Schema, GeneratedAt: now.UTC().Format(time.RFC3339), Findings: []FindingJSON{}, Machines: []EveryJSON{}}
	for _, v := range views {
		e := EveryJSON{Machine: machineOf(v), Body: body(v)}
		out.Findings = append(out.Findings, e.Findings...)
		if v.Silent != "" {
			e.Error = &ErrorJSON{Message: v.Silent}
		}
		out.Machines = append(out.Machines, e)
	}
	return out
}
