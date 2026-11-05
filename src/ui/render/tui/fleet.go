package tui

import (
	"strings"

	"github.com/jterrazz/studio-station/src/page"
	"github.com/jterrazz/studio-station/src/ui/atom"
	"github.com/jterrazz/studio-station/src/ui/token"
)

const (
	fleetMachine = 13
	fleetKind    = 11
	fleetCPU     = 11
	fleetMemory  = 14
	fleetDisk    = 13
	fleetNet     = 14
	fleetSpark   = 5
	fleetName    = 11
	fleetThermal = 19
)

type FleetRow struct {
	// value: a machine's row of All machines › Activity: VMs one step in
	// under their Mac, the last CPU percents it was read at, its rates; a
	// stopped one has none.
	Key                          string
	Mark                         page.Mark
	Depth                        int
	Name, Kind                   string
	Spark                        []int
	CPU, Memory, Disk, Net, Free string
	Stopped                      bool
}

type FleetMac struct {
	// value: a Mac's thermal state and battery, each with its mark.
	Name        string
	ThermalMark page.Mark
	Thermal     string
	BatteryMark page.Mark
	Battery     string
}

type Fleet struct {
	// value: All machines › Activity (SPEC §9.2): a row per machine, the
	// notes on how it was read, what needs you, each Mac's thermal state
	// and battery.
	Heads    []string
	Rows     []FleetRow
	Notes    []string
	Needs    Section
	Macs     *Section
	Thermals []FleetMac
	Cursor   Cursor
}

type Section struct {
	// value: a titled part of a pane: its title, its words, and what it
	// says when it has nothing.
	Title, Say string
	Checks     []Check
}

func (f Fleet) Lines() []string {
	st := token.New(f.Cursor.Dark)
	widths := []int{fleetMachine, fleetKind, fleetCPU, fleetMemory, fleetDisk, fleetNet}
	var head strings.Builder
	for i, h := range f.Heads {
		if i < len(widths) {
			head.WriteString(atom.Pad(h, widths[i]))
			continue
		}
		head.WriteString(h)
	}
	out := []Line{{Text: " " + st.Muted.Render(strings.TrimRight(head.String(), " "))}}
	for _, r := range f.Rows {
		name := atom.Spaces(2*r.Depth) + atom.Mark(st, r.Mark) + r.Name
		cpu, memory, disk, net := r.CPU, r.Memory, r.Disk, r.Net
		if r.Stopped {
			cpu, memory, disk, net = token.MarkNotFor, token.MarkNotFor, token.MarkNotFor, token.MarkNotFor
		} else if len(r.Spark) > 0 {
			cpu = atom.PadLeft(atom.Spark(r.Spark[max(len(r.Spark)-fleetSpark, 0):]), fleetSpark) + " " + atom.PadLeft(cpu, 3)
		}
		line := " " + atom.Pad(name, fleetMachine) + atom.Pad(r.Kind, fleetKind) + atom.Pad(cpu, fleetCPU) +
			atom.Pad(memory, fleetMemory) + atom.Pad(disk, fleetDisk) + atom.Pad(net, fleetNet) + r.Free
		out = append(out, Line{Key: r.Key, Text: line})
	}
	out = append(out, Line{})
	for _, n := range f.Notes {
		for _, l := range wrap(n, max(f.Cursor.Width-2, 20)) {
			out = append(out, Line{Text: " " + st.Muted.Render(l)})
		}
	}
	out = append(out, f.Needs.lines(st)...)
	if f.Macs != nil {
		out = append(out, f.Macs.lines(st)...)
		for _, m := range f.Thermals {
			out = append(out, Line{Text: " " + atom.Pad(m.Name, fleetName) + atom.Pad(atom.Mark(st, m.ThermalMark)+m.Thermal, fleetThermal) + atom.Mark(st, m.BatteryMark) + m.Battery})
		}
	}
	return f.Cursor.Lines(out)
}

func (s Section) lines(st token.Styles) []Line {
	out := []Line{{}, {Text: " " + st.Strong.Render(s.Title)}}
	for _, c := range s.Checks {
		out = append(out, Line{Text: " " + atom.Mark(st, c.Mark) + c.Label})
	}
	if len(s.Checks) == 0 && s.Say != "" {
		out = append(out, Line{Text: " " + s.Say})
	}
	return out
}
