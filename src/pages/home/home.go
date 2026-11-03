package home

import (
	"slices"
	"strconv"
	"strings"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/domain/config"
	"github.com/jterrazz/studio-station/src/domain/status"
	"github.com/jterrazz/studio-station/src/domain/tools"
	"github.com/jterrazz/studio-station/src/page"
	"github.com/jterrazz/studio-station/src/pages"
	pageconfig "github.com/jterrazz/studio-station/src/pages/config"
	pagetailnet "github.com/jterrazz/studio-station/src/pages/tailnet"
	pagetools "github.com/jterrazz/studio-station/src/pages/tools"
)

func Columns() []page.Column {
	return []page.Column{{Key: "name", Kind: page.KindText, Keep: 100}, {Key: "value", Kind: page.KindList, Keep: 60}}
}

func Snapshot(s status.Snapshot, here page.Context, all bool) page.Page {
	detail := []page.Section{
		section("machine", "Machine", machineRows(s)),
		section("environment", "Environment", environmentRows(s)),
		section("workspace", "Workspace", workspaceRows(s.Workspace)),
		section("identity", "Identity", checkRows(s.Identity)),
	}
	if len(s.Daemons) > 0 {
		detail = append(detail, section("daemons", "Daemons", checkRows(s.Daemons)))
	}
	if len(s.Gateways) > 0 {
		detail = append(detail, section("gateways", "Gateways", checkRows(s.Gateways)))
	}
	detail = append(detail, toolsSection(s), configSection(s))
	fold := page.Folded
	if all {
		fold = page.Open
	}
	for i := range detail {
		detail[i].Fold = fold
	}
	sealed := page.Seal(page.Page{Sections: detail})
	p := page.Page{
		ID:      "home",
		Command: "status",
		Context: here,
		Hints: []page.Hint{
			{Command: "station status --all", Text: "for every row"},
			{Command: "station status | cat", Text: "for records", External: true},
		},
	}
	if all {
		p.Hints = p.Hints[1:]
	}
	p.Sections = append([]page.Section{todo(sealed.Sections), summary(s, sealed.Sections)}, sealed.Sections...)
	return page.Seal(p)
}

func section(id, title string, rows []page.Row) page.Section {
	items := make([]page.Item, 0, len(rows))
	for i := range rows {
		items = append(items, page.Item{Row: &rows[i]})
	}
	return page.Section{ID: id, Title: title, Columns: Columns(), Items: items}
}

func toolsSection(s status.Snapshot) page.Section {
	if s.ToolsPending {
		return section("tools", "Tools", []page.Row{pending("catalogue", "catalogue")})
	}
	var items []page.Item
	for _, r := range s.Tools {
		if r.State != catalog.StateOK {
			row := pagetools.Row(r)
			items = append(items, page.Item{Row: &row})
		}
	}
	return page.Section{ID: "tools", Title: "Tools", Columns: pagetools.Columns(), Items: items, Uncounted: true, Counts: toolCounts(s.Tools)}
}

func configSection(s status.Snapshot) page.Section {
	if s.ConfigPending {
		return section("config", "Config", []page.Row{pending("items", "items")})
	}
	var items []page.Item
	for _, r := range s.Config {
		if r.State != catalog.StateOK {
			row := pageconfig.Row(r)
			items = append(items, page.Item{Row: &row})
		}
	}
	return page.Section{ID: "config", Title: "Config", Columns: pageconfig.Columns(), Items: items, Uncounted: true, Counts: configCounts(s.Config)}
}

func tallies(marks []page.Mark) []page.Tally {
	var out []page.Tally
	for _, m := range page.Marks() {
		n := 0
		for _, x := range marks {
			if x == m {
				n++
			}
		}
		if n > 0 {
			out = append(out, page.Tally{Mark: m, N: n})
		}
	}
	return out
}

func toolCounts(reports []tools.Report) []page.Tally {
	marks := make([]page.Mark, 0, len(reports))
	for _, r := range reports {
		marks = append(marks, pages.MarkOf(r.State))
	}
	return tallies(marks)
}

func configCounts(reports []config.Report) []page.Tally {
	marks := make([]page.Mark, 0, len(reports))
	for _, r := range reports {
		marks = append(marks, pages.MarkOf(r.State))
	}
	return tallies(marks)
}

func reasonOf(m page.Mark) page.Reason {
	switch m {
	case page.Attention:
		return page.Degraded
	case page.Broken:
		return page.Down
	case page.Absent:
		return page.Missing
	case page.OK, page.Awaiting, page.Unjudged:
	}
	return ""
}

func row(id string, mark page.Mark, value page.Value, detail string) page.Row {
	return page.Row{ID: id, Label: strings.ReplaceAll(id, "-", " "), Mark: mark, Reason: reasonOf(mark), Cells: []page.Value{value}, Detail: page.Text(detail)}
}

// invariant: a pending row says only its name: its value is the probe's, and
// the probe has not answered.
func pending(id, label string) page.Row {
	return page.Row{ID: id, Label: label, Mark: page.Awaiting}
}

// value: a protection the owner turned off on purpose is a value, not an absence (M3).
func offByChoice(c status.Check) bool {
	return c.State == catalog.StateAbsent && c.Value == "off" && slices.Contains([]string{"sip", "remote-login", "vpn"}, c.Name)
}

func checkRows(checks []status.Check) []page.Row {
	rows := make([]page.Row, 0, len(checks))
	for _, c := range checks {
		if c.State == catalog.StatePending {
			rows = append(rows, pending(c.Name, strings.ReplaceAll(c.Name, "-", " ")))
			continue
		}
		value := page.Text(c.Value)
		if c.Value == "" && c.State == catalog.StateUnknown {
			value = page.Text("unknown")
		}
		mark := pages.MarkOf(c.State)
		if offByChoice(c) {
			mark, value = page.OK, page.Off()
		}
		rows = append(rows, row(c.Name, mark, value, c.Detail))
	}
	return rows
}

func machineRows(s status.Snapshot) []page.Row {
	m := s.Machine
	facts := page.Texts(string(m.Kind), strings.TrimSpace(m.OS+" "+m.Arch), m.User, m.Shell)
	name := page.Row{ID: "name", Label: "name", Mark: page.OK, Cells: []page.Value{page.Text(m.Name)}, Detail: facts}
	if !m.Registered {
		name.Mark, name.Cells = page.Absent, []page.Value{page.Text("unregistered")}
	}
	rows := []page.Row{name}
	if s.System.Pending {
		rows = append(rows, pending("uptime", "uptime"))
	}
	if s.System.Uptime != "" {
		detail := ""
		if s.System.Battery != "" {
			detail = "battery " + s.System.Battery
		}
		rows = append(rows, row("uptime", page.OK, page.Text(s.System.Uptime), detail))
	}
	rows = append(rows, setupRow(s.Setup))
	if d := s.Station.Daemon; d.State == catalog.StatePending {
		rows = append(rows, pending("daemon", "daemon"))
	} else if d.Value != "" {
		rows = append(rows, row("daemon", pages.MarkOf(d.State), page.Text(d.Value), d.Detail))
	}
	return rows
}

func setupRow(s status.Setup) page.Row {
	if s.State == catalog.StatePending {
		return pending("setup", "setup")
	}
	fix := page.Cmd("station setup")
	for _, step := range s.Steps {
		if step.State == catalog.StateOK {
			continue
		}
		r := page.Row{ID: "setup", Label: "setup", Mark: page.Attention, Reason: page.Partial, Cells: []page.Value{page.Text(step.Name)}, Hint: fix}
		if step.Name == "name" {
			r.Reason, r.Detail = page.Unset, page.Text("not set")
		}
		return r
	}
	if s.State != catalog.StateOK {
		return page.Row{ID: "setup", Label: "setup", Mark: pages.MarkOf(s.State), Reason: page.Late, Cells: []page.Value{page.Text("unknown")}, Hint: fix}
	}
	return row("setup", page.OK, page.Text("done"), "")
}

func environmentRows(s status.Snapshot) []page.Row {
	rows := checkRows(s.Environment.Network)
	rows = append(rows, checkRows(s.Environment.Security)...)
	rows = append(rows, checkRows(s.Environment.Updates)...)
	if s.Tailnet.State == catalog.StatePending {
		return append(rows, pending("tailnet", "tailnet"))
	}
	return append(rows, pagetailnet.Summary(s.Tailnet))
}

func workspaceRows(w status.Workspace) []page.Row {
	rows := make([]page.Row, 0, 4+len(w.Strays))
	rows = append(rows,
		pendingOr(w.ReposPending, "repos", reposRow(w.Repositories)),
		pendingOr(w.DepsPending, "deps", depsRow(w.Dependencies)),
		pendingOr(w.Docker.State == catalog.StatePending, "docker", dockerRow(w.Docker)),
		diskRow(w.Disk, w.DiskPending))
	for i, st := range w.Strays {
		r := page.Row{ID: st.Name + "/" + strconv.Itoa(i), Label: st.Name, Mark: page.Attention, Reason: page.Stray, Cells: []page.Value{page.Path(st.Path)}, Detail: page.Size(st.Bytes)}
		if st.Repair != "" {
			r.Hint = page.Cmd(st.Repair)
		}
		rows = append(rows, r)
	}
	return rows
}

func pendingOr(waiting bool, name string, answered page.Row) page.Row {
	if waiting {
		return pending(name, name)
	}
	return answered
}

func reposRow(repos []status.Repository) page.Row {
	if len(repos) == 0 {
		return row("repos", page.Absent, page.Text("none"), "~/Developer")
	}
	dirty := 0
	for _, r := range repos {
		dirty += r.Dirty
	}
	r := row("repos", page.OK, page.Count(int64(len(repos)), ""), "")
	r.Detail = page.Count(int64(dirty), "dirty")
	return r
}

func depsRow(deps []status.Dependency) page.Row {
	if len(deps) == 0 {
		return row("deps", page.Absent, page.Text("none"), "")
	}
	outdated, repos, unscanned := 0, 0, 0
	for _, d := range deps {
		switch {
		case d.Outdated < 0:
			unscanned++
		case d.Outdated > 0:
			outdated += d.Outdated
			repos++
		}
	}
	late := page.None()
	if unscanned > 0 {
		late = page.Count(int64(unscanned), "not scanned in time")
	}
	if outdated == 0 {
		r := row("deps", page.OK, page.Text("up to date"), "")
		r.Detail = late
		return r
	}
	r := row("deps", page.Attention, page.Count(int64(outdated), "outdated"), "")
	r.Reason = page.Outdated
	r.Detail = page.Text("in " + strconv.Itoa(repos) + " repos")
	if !late.IsNone() {
		r.Detail = page.List(r.Detail, late)
	}
	return r
}

func dockerRow(d status.Docker) page.Row {
	switch d.State {
	case catalog.StateOK:
		r := row("docker", page.OK, page.Text("running"), "")
		r.Detail = page.List(page.Count(int64(d.Containers), "containers"), page.Count(int64(d.Images), "images"))
		return r
	case catalog.StateAbsent:
		r := row("docker", page.Absent, page.Text("not installed"), "")
		r.Hint = page.Cmd("station tools install docker")
		return r
	case catalog.StateDegraded:
		return row("docker", page.OK, page.Off(), "")
	case catalog.StateBroken, catalog.StateUnknown, catalog.StatePending:
	}
	r := row("docker", page.Attention, page.Text("unknown"), "")
	r.Reason = page.Late
	return r
}

func diskRow(disk []status.Disk, waiting bool) page.Row {
	var total int64
	for _, d := range disk {
		if d.Level == catalog.LevelCache {
			total += d.Bytes
		}
	}
	if waiting {
		last := page.Pending()
		if total > 0 {
			last = page.Stale(page.Size(total))
		}
		return page.Row{ID: "disk", Label: "disk", Mark: page.Awaiting, Cells: []page.Value{last}}
	}
	if total == 0 {
		return row("disk", page.OK, page.Text("no caches"), "")
	}
	return row("disk", page.OK, page.Size(total), "reclaimable: station storage clean")
}

func rowsOf(s page.Section) []page.Row {
	var out []page.Row
	for _, it := range s.Items {
		if it.Row != nil {
			out = append(out, *it.Row)
		}
	}
	return out
}

func todo(detail []page.Section) page.Section {
	var broken, attention []page.Row
	for _, s := range detail {
		rows := rowsOf(s)
		if s.ID == "tools" {
			rows = foldTools(rows)
		}
		for _, r := range rows {
			act := r
			act.ID = s.ID + "/" + r.ID
			if act.Detail.IsNone() && len(r.Cells) > 0 {
				act.Detail = r.Cells[0]
			}
			act.Cells = nil
			switch r.Mark {
			case page.Broken:
				broken = append(broken, act)
			case page.Attention:
				attention = append(attention, act)
			case page.OK, page.Absent, page.Awaiting, page.Unjudged:
			}
		}
	}
	items := make([]page.Item, 0, len(broken)+len(attention))
	for _, list := range [][]page.Row{broken, attention} {
		for i := range list {
			items = append(items, page.Item{Row: &list[i]})
		}
	}
	if len(items) == 0 {
		items = append(items, page.Item{Empty: &page.Empty{Noun: "actions"}})
	}
	return page.Section{ID: "todo", Title: "To do", Role: page.Actions, Fold: page.Open, Columns: []page.Column{{Key: "name", Kind: page.KindText, Keep: 100}}, Items: items}
}

func foldTools(rows []page.Row) []page.Row {
	type key struct {
		reason page.Reason
		fix    string
	}
	groups := map[key][]page.Row{}
	var order []key
	for _, r := range rows {
		if r.Mark != page.Attention {
			continue
		}
		k := key{reason: r.Reason}
		if r.Hint != nil {
			k.fix = r.Hint.Command
		}
		if _, seen := groups[k]; !seen {
			order = append(order, k)
		}
		groups[k] = append(groups[k], r)
	}
	out := make([]page.Row, 0, len(order))
	for _, k := range order {
		g := groups[k]
		if len(g) == 1 {
			out = append(out, g[0])
			continue
		}
		n := int64(len(g))
		r := page.Row{ID: string(k.reason) + "/" + k.fix, Label: "tools", Mark: page.Attention, Reason: k.reason, Hint: g[0].Hint}
		switch k.reason {
		case page.Missing:
			r.Detail = page.Count(n, "wait for "+strings.TrimPrefix(k.fix, "station tools install "))
		case page.Unmanaged:
			r.Detail = page.Count(n, "on PATH, not managed by their method")
			r.Hint = page.Cmd("station tools")
		default:
			r.Detail = page.Count(n, string(k.reason))
		}
		out = append(out, r)
	}
	return out
}

func worst(s page.Section) page.Mark {
	for _, m := range []page.Mark{page.Broken, page.Attention, page.Awaiting} {
		for _, t := range s.Counts {
			if t.Mark == m {
				return m
			}
		}
	}
	return page.OK
}

func find(sections []page.Section, id string) page.Section {
	for _, s := range sections {
		if s.ID == id {
			return s
		}
	}
	return page.Section{}
}

func summary(s status.Snapshot, detail []page.Section) page.Section {
	area := func(id, label string, mark page.Mark, value page.Value) page.Item {
		return page.Item{Row: &page.Row{ID: "summary/" + id, Label: label, Mark: mark, Cells: []page.Value{value}}}
	}
	counts := func(id string) page.Value {
		sec := find(detail, id)
		parts := make([]page.Value, 0, len(sec.Counts))
		for _, t := range sec.Counts {
			parts = append(parts, page.Labeled(string(t.Mark), page.Count(int64(t.N), "")))
		}
		return page.List(parts...)
	}
	m := s.Machine
	machine := page.Texts(string(m.Kind), strings.TrimSpace(m.OS+" "+m.Arch))
	if s.System.Uptime != "" {
		machine = page.Texts(string(m.Kind), strings.TrimSpace(m.OS+" "+m.Arch), "up "+s.System.Uptime)
	}
	tailnet := page.Text(s.Tailnet.Name)
	identity := page.None()
	for _, c := range s.Identity {
		if c.Value != "" {
			identity = page.Text(c.Value)
			break
		}
	}
	repos := page.Count(int64(len(s.Workspace.Repositories)), "repos")
	var cache int64
	for _, d := range s.Workspace.Disk {
		if d.Level == catalog.LevelCache {
			cache += d.Bytes
		}
	}
	storage := page.Size(cache)
	storage.Unit = "reclaimable"
	if n := len(s.Workspace.Strays); n > 0 {
		storage = page.List(storage, page.Count(int64(n), "strays"))
	}
	fleet := page.Text(s.Station.Daemon.Value)
	items := []page.Item{
		area("machine", "Machine", worst(find(detail, "machine")), machine),
		area("network", "Network", worst(find(detail, "environment")), counts("environment")),
		area("tailnet", "Tailnet", pages.MarkOf(s.Tailnet.State), tailnet),
		area("identity", "Identity", worst(find(detail, "identity")), identity),
		area("workspace", "Workspace", worst(find(detail, "workspace")), repos),
	}
	if len(s.Daemons) > 0 {
		items = append(items, area("daemons", "Daemons", worst(find(detail, "daemons")), counts("daemons")))
	}
	items = append(items,
		area("tools", "Tools", worst(find(detail, "tools")), counts("tools")),
		area("config", "Config", worst(find(detail, "config")), counts("config")),
		area("storage", "Storage", page.OK, storage),
		area("fleet", "Fleet", pages.MarkOf(s.Station.Daemon.State), fleet),
	)
	if len(s.Gateways) > 0 {
		items = append(items, area("gateways", "Gateways", worst(find(detail, "gateways")), counts("gateways")))
	}
	return page.Section{ID: "summary", Title: "Summary", Role: page.Summary, Fold: page.Open, Columns: []page.Column{{Key: "area", Kind: page.KindText, Keep: 100}, {Key: "value", Kind: page.KindList, Keep: 60}}, Items: items}
}

func Live(p page.Page, samples []status.Sample, memoryTotal int64) page.Page {
	if len(samples) == 0 {
		return p
	}
	last := samples[len(samples)-1]
	strip := page.Strip{GPU: page.Percent(int64(last.GPU * 10))}
	cpu := func(s status.Sample) float64 {
		total := 0.0
		for _, proc := range s.CPU {
			total += proc.Value
		}
		return min(total, 100)
	}
	strip.CPU = page.Percent(int64(cpu(last) * 10))
	for _, s := range samples {
		strip.Spark = append(strip.Spark, int(cpu(s)))
	}
	if len(strip.Spark) > 16 {
		strip.Spark = strip.Spark[len(strip.Spark)-16:]
	}
	var used int64
	for _, proc := range last.Memory {
		used += int64(proc.Value)
	}
	if memoryTotal > 0 {
		strip.Memory = page.Ratio(min(used, memoryTotal), memoryTotal)
	}
	if len(samples) > 1 {
		prev := samples[len(samples)-2]
		if dt := last.At.Sub(prev.At).Seconds(); dt > 0 {
			rx, tx := page.Size(int64(float64(max(last.NetRx-prev.NetRx, 0))/dt)), page.Size(int64(float64(max(last.NetTx-prev.NetTx, 0))/dt))
			rx.Unit, tx.Unit = "/s", "/s"
			strip.Net = []page.Value{rx, tx}
		}
	}
	var items []page.Item
	for i, proc := range last.CPU {
		if i == 5 {
			break
		}
		items = append(items, page.Item{Row: &page.Row{ID: "cpu/" + strconv.Itoa(proc.PID), Label: proc.Name, Cells: []page.Value{page.Text("cpu"), page.Percent(int64(proc.Value * 10))}}})
	}
	for i, proc := range last.Memory {
		if i == 5 {
			break
		}
		items = append(items, page.Item{Row: &page.Row{ID: "memory/" + strconv.Itoa(proc.PID), Label: proc.Name, Cells: []page.Value{page.Text("memory"), page.Size(int64(proc.Value))}}})
	}
	if len(items) > 0 {
		top := page.Section{ID: "processes", Title: "Processes", Fold: page.Folded, Caption: []page.Value{page.Text("top 5 cpu, memory")},
			Columns: []page.Column{{Key: "name", Kind: page.KindListing, Keep: 100}, {Key: "measure", Kind: page.KindText, Keep: 50}, {Key: "value", Kind: page.KindText, Keep: 60}}, Items: items}
		at := min(2, len(p.Sections))
		p.Sections = append(p.Sections[:at:at], append([]page.Section{top}, p.Sections[at:]...)...)
	}
	p.Strip = &strip
	return page.Seal(p)
}
