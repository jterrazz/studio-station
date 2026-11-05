package machines

import (
	"cmp"
	"slices"
	"strconv"
	"strings"

	"github.com/jterrazz/studio-station/src/page"
)

type TableIn struct {
	Machines page.Page
	// value: the pages of Activity and Ports across every machine; nil
	// while unread.
	Activity, Ports *page.Page
	// value: each Mac's Overview, by its name; absent while unread.
	Overviews map[string]page.Page
	// value: the CPU percent of each machine at each read of Activity this
	// session made, oldest first.
	Samples map[string][]int
}

func tableColumns() []page.Column {
	// invariant: a short line leaves out DISK, then MEMORY, then the rest by
	// keep, whole, and says so (SPEC §9.9); the machine, its kind and its
	// need are kept the longest.
	return []page.Column{
		{Key: "name", Label: "MACHINE", Kind: page.KindText, Keep: 100},
		{Key: "kind", Label: "KIND", Kind: page.KindText, Keep: 90},
		{Key: "cpu", Label: "CPU", Kind: page.KindText, Keep: 80, Whole: true},
		{Key: "memory", Label: "MEMORY", Kind: page.KindText, Keep: 30, Whole: true},
		{Key: "disk", Label: "DISK", Kind: page.KindText, Keep: 20, Whole: true},
		{Key: "ports", Label: "PORTS", Kind: page.KindText, Keep: 70, Whole: true},
		{Key: "awake", Label: "AWAKE", Kind: page.KindText, Keep: 60, Whole: true},
		{Key: "need", Label: "NEED", Kind: page.KindText, Keep: 85, Whole: true},
	}
}

func Table(in TableIn) page.Page {
	// invariant: the All machines table of the TUI (SPEC §9.2, MA-13) is the
	// machines page joined, by each machine's name, with the pages of the
	// reads it shows, each the page its own command prints: Activity and
	// Ports across every machine, and each Mac's Overview for its Awake and
	// its Needs you. A cell whose read has not answered waits, never guesses.
	p := page.Page{ID: "all-machines", Command: "machines", Context: in.Machines.Context}
	var items []page.Item
	var counts []page.Value
	labels := map[string]string{}
	for _, sec := range in.Machines.Sections {
		if sec.ID != "machines" {
			continue
		}
		for _, v := range sec.Caption {
			if t := v.Text; strings.HasSuffix(t, " running") || strings.HasSuffix(t, " stopped") {
				counts = append(counts, v)
			}
		}
		for _, it := range sec.Items {
			if it.Group == nil {
				continue
			}
			h := it.Group.Head
			labels[h.ID] = cmp.Or(h.Short, h.Label)
			g := page.Group{Head: in.row(h, "Mac", true), Fold: page.Open}
			for _, vi := range it.Group.Items {
				if vi.Row == nil {
					g.Items = append(g.Items, vi)
					continue
				}
				r := in.row(*vi.Row, cell(sec, *vi.Row, "kind").Text, false)
				g.Items = append(g.Items, page.Item{Row: &r})
			}
			items = append(items, page.Item{Group: &g})
		}
	}
	p.Sections = append(p.Sections, page.Section{ID: "machines", Uncounted: true, SaysDropped: true, Columns: tableColumns(), Items: items})
	counts = append(counts, page.Text("PORTS: public"), page.Text("private"))
	p.Sections = append(p.Sections, page.Section{ID: "counts", Caption: counts})
	if needs := in.needs(labels); len(needs) > 0 {
		p.Sections = append(p.Sections, page.Section{ID: "needs-you", Title: "Needs you", Caption: []page.Value{page.Text(strconv.Itoa(len(needs)))}, Joined: true, Uncounted: true,
			Columns: []page.Column{{Key: "machine", Kind: page.KindText, Keep: 100}, {Key: "finding", Kind: page.KindText, Keep: 90}, {Key: "fix", Kind: page.KindList, Keep: 50}}, Items: needs})
	}
	return page.Seal(p)
}

func cell(sec page.Section, r page.Row, key string) page.Value {
	for i, c := range sec.Columns {
		if c.Key == key && i > 0 && i-1 < len(r.Cells) {
			return r.Cells[i-1]
		}
	}
	return page.None()
}

func waits() page.Value { return page.Marked(page.Awaiting) }

// value: a cell with no value on this row (a stopped VM's CPU, a VM's
// Awake, which follows its Mac): the dash, never blank.
func none() page.Value { return page.Marked(page.NotForThisMachine) }

func rowIn(p page.Page, sectionID, rowID string) (page.Section, page.Row, bool) {
	for _, sec := range p.Sections {
		if sec.ID != sectionID {
			continue
		}
		for _, it := range sec.Items {
			if it.Row != nil && it.Row.ID == rowID {
				return sec, *it.Row, true
			}
		}
	}
	return page.Section{}, page.Row{}, false
}

func (in TableIn) row(m page.Row, kind string, mac bool) page.Row {
	r := page.Row{ID: m.ID, Label: cmp.Or(m.Short, m.Label), Mark: m.Mark}
	stopped := m.Mark == page.Absent
	r.Cells = []page.Value{page.Text(kind), in.activity(m.ID, "cpu", stopped), in.activity(m.ID, "memory", stopped), in.activity(m.ID, "free", stopped), in.ports(m.ID, stopped)}
	ov, read := in.Overviews[m.ID]
	switch {
	case !mac:
		r.Cells = append(r.Cells, none(), none())
	case !read:
		r.Cells = append(r.Cells, waits(), waits())
	default:
		r.Cells = append(r.Cells, awake(ov), page.Text(strconv.Itoa(len(needsOf(ov)))))
	}
	return r
}

func (in TableIn) activity(name, key string, stopped bool) page.Value {
	if in.Activity == nil {
		return waits()
	}
	sec, r, found := rowIn(*in.Activity, "machines", name)
	if !found || stopped || r.Mark == page.Absent {
		return none()
	}
	v := cell(sec, r, key)
	if v.IsNone() {
		return none()
	}
	if key == "cpu" && len(in.Samples[name]) > 0 {
		samples := in.Samples[name]
		return page.Spark(samples, page.Text(strconv.Itoa(samples[len(samples)-1])+"%"))
	}
	return v
}

func (in TableIn) ports(name string, stopped bool) page.Value {
	if in.Ports == nil {
		return waits()
	}
	if stopped {
		return none()
	}
	public, private := 0, 0
	for _, sec := range in.Ports.Sections {
		for _, it := range sec.Items {
			if it.Row == nil {
				continue
			}
			if on, _, _ := strings.Cut(it.Row.ID, ":"); on != name {
				continue
			}
			switch cell(sec, *it.Row, "class").Text {
			case "public":
				public++
			case "private":
				private++
			}
		}
	}
	return page.List(page.Text(strconv.Itoa(public)), page.Text(strconv.Itoa(private)))
}

func awake(ov page.Page) page.Value {
	_, r, found := rowIn(ov, "sections", "awake")
	if !found || len(r.Cells) == 0 {
		return none()
	}
	c := r.Cells[0]
	if len(c.Items) > 0 {
		c = c.Items[0]
	}
	return page.Text(strings.TrimSuffix(c.Text, " normally"))
}

func needsOf(ov page.Page) []page.Row {
	var out []page.Row
	for _, sec := range ov.Sections {
		if sec.ID != "needs-you" {
			continue
		}
		for _, it := range sec.Items {
			if it.Row != nil {
				out = append(out, *it.Row)
			}
		}
	}
	return out
}

func (in TableIn) needs(labels map[string]string) []page.Item {
	// invariant: Needs you across every machine is each Mac's own Needs you,
	// named by its Mac, then the machines page's own (a Mac that does not
	// answer); broken first, then in the machines page's order, This Mac
	// first (SPEC §5.1.1). A row's ID leads with its machine's name.
	var rows []page.Row
	for _, sec := range in.Machines.Sections {
		if sec.ID != "machines" {
			continue
		}
		for _, it := range sec.Items {
			if it.Group == nil {
				continue
			}
			name := it.Group.Head.ID
			for _, n := range needsOf(in.Overviews[name]) {
				fix := page.None()
				if len(n.Cells) > 0 {
					fix = n.Cells[len(n.Cells)-1]
				}
				rows = append(rows, page.Row{ID: name + ":" + n.ID, Label: labels[name], Mark: n.Mark, Cells: []page.Value{page.Text(n.Label), fix}})
			}
		}
	}
	for _, n := range needsOf(in.Machines) {
		name := n.Label
		for id, label := range labels {
			if label == n.Label {
				name = id
			}
		}
		n.ID = name + ":machines-" + n.ID
		rows = append(rows, n)
	}
	slices.SortStableFunc(rows, func(a, b page.Row) int {
		return cmp.Compare(rank(b.Mark), rank(a.Mark))
	})
	out := make([]page.Item, len(rows))
	for i := range rows {
		out[i] = page.Item{Row: &rows[i]}
	}
	return out
}

func rank(m page.Mark) int {
	if m == page.Broken {
		return 1
	}
	return 0
}
