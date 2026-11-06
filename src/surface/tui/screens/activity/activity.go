package activity

import (
	"cmp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/jterrazz/studio-station/src/core/section"
	"github.com/jterrazz/studio-station/src/page"
	pageactivity "github.com/jterrazz/studio-station/src/pages/activity"
	"github.com/jterrazz/studio-station/src/surface/tui/screens/screen"
	ui "github.com/jterrazz/studio-station/src/ui/render/tui"
)

type window struct {
	flag, words string
}

// value: SPEC §6.4's picker, `w` cycling it: 2 s points, then minutes.
func windows() []window { return []window{{"6m", "6 min"}, {"1h", "1 h"}, {"24h", "24 h"}} }

type Screen struct{}

func (Screen) Live() time.Duration { return 2 * time.Second }

func of(st screen.State) window {
	for _, w := range windows() {
		if w.flag == st["window"] {
			return w
		}
	}
	return windows()[0]
}

func (Screen) Words(_ screen.Machine, words []string, st screen.State) []string {
	return append(slices.Clone(words), "--graph", of(st).flag)
}

func (Screen) Skips(m screen.Machine, s page.Section) bool {
	// value: All machines' cursor walks the machines, never the lines
	// under them.
	return m.Kind == "" && s.ID != "machines"
}

func banner(p page.Page) []string {
	for _, s := range p.Sections {
		if s.Role == page.Banner && s.ID != page.OnSection {
			for _, it := range s.Items {
				if it.Row != nil && len(it.Row.Cells) > 0 {
					var words []string
					for _, w := range it.Row.Cells[0].Items {
						words = append(words, ui.Plain(w))
					}
					if len(words) == 0 {
						words = append(words, ui.Plain(it.Row.Cells[0]))
					}
					return words
				}
			}
		}
	}
	return nil
}

func (Screen) Title(v screen.View) string {
	if v.Machine.Kind == "" {
		n := 0
		for _, s := range v.Page.Sections {
			if s.ID == "machines" {
				n = len(s.Items)
			}
		}
		return "Activity" + ui.Sep + "all machines" + ui.Sep + strconv.Itoa(n)
	}
	title := "Activity" + ui.Sep + v.Machine.Label
	for _, w := range banner(v.Page) {
		if strings.HasPrefix(w, "from ") {
			return title + ui.Sep + w
		}
	}
	return title + ui.Sep + "last " + of(v.State).words
}

func (Screen) Keys(v screen.View) []ui.Binding {
	if v.Machine.Kind == "" {
		return []ui.Binding{screen.Bind("enter", "open its Activity")}
	}
	return []ui.Binding{screen.Bind("w", "window"), screen.Bind("p", "sort processes")}
}

func (Screen) Press(key string, v screen.View) (screen.Effect, bool) {
	switch {
	case v.Machine.Kind == "" && key == "enter" && v.HasRow && v.Of.ID == "machines":
		return screen.Effect{Open: &screen.Open{Machine: v.Row.ID, Section: section.Activity}}, true
	case v.Machine.Kind == "":
		return screen.Effect{}, false
	case key == "w":
		all := windows()
		i := slices.Index(all, of(v.State))
		return screen.Effect{State: v.State.With("window", all[(i+1)%len(all)].flag), Read: true}, true
	case key == "p":
		sort := "memory"
		if v.State["sort"] == "memory" {
			sort = "cpu"
		}
		return screen.Effect{State: v.State.With("sort", sort)}, true
	}
	return screen.Effect{}, false
}

func find(p page.Page, id string) (page.Section, bool) {
	for _, s := range p.Sections {
		if s.ID == id {
			return s, true
		}
	}
	return page.Section{}, false
}

func (Screen) Lines(v screen.View) ([]string, bool) {
	if v.Machine.Kind == "" {
		return allMachines(v)
	}
	graphs, ok := find(v.Page, pageactivity.Graphs)
	if !ok {
		return nil, false
	}
	a := ui.Activity{Banner: banner(v.Page), Graphs: graphs, Sort: v.State["sort"], Width: v.Width, Dark: v.Dark, Sampled: sampled(v)}
	a.Values, _ = find(v.Page, "values")
	if procs, ok := find(v.Page, pageactivity.Processes); ok {
		a.Processes = &procs
	}
	return a.Lines(), true
}

// value: SPEC §9.2: a machine of this Mac is sampled by this Mac's engine,
// every 2 s while the TUI shows it, so its 6 min window is drawn from 2 s
// points and its longer ones from 60 s points; another Mac's machine is
// that Mac's engine's to sample, and its banner stays.
func sampled(v screen.View) string {
	m := v.Machine
	if !m.ThisMachine && !m.MacHere {
		return ""
	}
	if of(v.State).flag == windows()[0].flag {
		return "Sampled every 2 s by the engine while you watch"
	}
	return "Sampled every 60 s by the engine"
}

// value: SPEC §9.2's All machines › Activity: a row per machine, each VM
// under its Mac with the CPU percents the TUI read it at, how the rates
// were read, what needs you from them, and each Mac's thermal state and
// battery.
func allMachines(v screen.View) ([]string, bool) {
	machines, ok := find(v.Page, "machines")
	if !ok {
		return nil, false
	}
	f := ui.AllMachines{Cursor: ui.Cursor{Key: v.Cursor, Focused: v.Focused, Dark: v.Dark, NoColor: v.NoColor, Width: v.Width, Room: v.Room}}
	f.Heads = []string{"MACHINE"}
	for _, c := range machines.Columns[1:] {
		f.Heads = append(f.Heads, c.Label)
	}
	var stopped, silent []string
	for _, it := range machines.Items {
		r := it.Row
		if r == nil || (v.Match != nil && !v.Match(*r)) {
			continue
		}
		name := cmp.Or(r.Short, r.Label)
		row := ui.AllMachinesRow{Key: machines.ID + "/" + r.ID, Mark: r.Mark, Name: name, Kind: ui.Plain(screen.Cell(machines, *r, "kind")), Spark: v.Samples[r.ID]}
		if row.Kind != "Mac" {
			row.Depth = 1
		}
		row.CPU = strings.ReplaceAll(ui.Plain(screen.Cell(machines, *r, "cpu")), " %", "%")
		row.Memory, row.Disk = ui.Plain(screen.Cell(machines, *r, "memory")), ui.Plain(screen.Cell(machines, *r, "disk"))
		row.Net, row.Free = ui.Plain(screen.Cell(machines, *r, "net")), ui.Plain(screen.Cell(machines, *r, "free"))
		switch r.Mark {
		case page.Absent:
			row.Stopped, row.Spark = true, nil
			stopped = append(stopped, name)
		case page.Broken:
			row.Spark = nil
			silent = append(silent, name+" "+row.CPU)
		default:
		}
		f.Rows = append(f.Rows, row)
	}
	f.Notes = append(f.Notes, rates(v.Page)+"FREE: the machine's own disk; a VM's from inside it.")
	var said []string
	switch len(stopped) {
	case 0:
	case 1:
		said = append(said, stopped[0]+" is stopped.")
	default:
		said = append(said, strings.Join(stopped[:len(stopped)-1], ", ")+" and "+stopped[len(stopped)-1]+" are stopped.")
	}
	for _, s := range silent {
		said = append(said, s+".")
	}
	f.Notes = append(f.Notes, strings.Join(append(said, "Every value comes from the machine itself (guest inside a VM)."), " "))
	needs, _ := find(v.Page, "needs")
	f.Needs = ui.Section{Title: "NEEDS YOU" + ui.Sep + strconv.Itoa(len(needs.Items)) + " from activity", Say: "Nothing is hot, full or swapping."}
	for _, it := range needs.Items {
		if r := it.Row; r != nil {
			f.Needs.Checks = append(f.Needs.Checks, ui.Check{Mark: r.Mark, Label: r.Label})
		}
	}
	if macs, ok := find(v.Page, pageactivity.Macs); ok {
		f.Macs = &ui.Section{Title: "THERMAL AND BATTERY" + ui.Sep + "Macs only"}
		for _, it := range macs.Items {
			r := it.Row
			if r == nil {
				continue
			}
			m := ui.AllMachinesMac{Name: cmp.Or(r.Short, r.Label), ThermalMark: r.Mark, Thermal: ui.Plain(screen.Cell(macs, *r, "thermal")), BatteryMark: page.OK}
			if b := screen.Cell(macs, *r, "battery"); b.IsNone() {
				m.BatteryMark, m.Battery = page.NotForThisMachine, "no battery"
			} else {
				m.Battery = ui.Plain(b)
			}
			f.Thermals = append(f.Thermals, m)
		}
	}
	return f.Lines(), true
}

// value: how the page's rates were read, in the screen's words (`Rates:
// last 10 s. `); empty while none is.
func rates(p page.Page) string {
	for _, w := range banner(p) {
		if r, ok := strings.CutPrefix(w, "rates over "); ok {
			return "Rates: last " + strings.TrimPrefix(r, "the last ") + ". "
		}
	}
	return ""
}
