package machines

import (
	"strconv"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/domain/machines"
	"github.com/jterrazz/studio-station/src/present"
)

func Fleet(f machines.Fleet, warnings []string, context string) present.Answer {
	// invariant: one column schema for every row, host or VM, and no row ends on an
	// empty cell, so a piped row always has the same number of fields.
	hereVersion := ""
	for _, h := range f.Hosts {
		if h.Here {
			hereVersion = h.StationVersion
		}
	}
	var rows []present.Row
	for _, h := range f.Hosts {
		rows = append(rows, hostRow(h, hereVersion))
		for _, vm := range h.VMs {
			rows = append(rows, vmRow(vm))
		}
	}
	blocks := make([]present.Block, 0, 1+len(warnings))
	blocks = append(blocks, present.Table{Rows: rows})
	for _, w := range warnings {
		blocks = append(blocks, present.Notice{Tone: present.Warn, Text: w})
	}
	return present.Answer{
		Doc:  present.Document{Command: "station machines", Context: context, Blocks: blocks},
		JSON: machines.ToWire(f),
	}
}

func hostRow(h machines.HostView, hereVersion string) present.Row {
	mark, state, tone := present.MarkOK, "online", present.Good
	resources := plural(len(h.VMs), "VM")
	switch {
	case !h.Reachable:
		mark, state, tone, resources = present.MarkDegraded, "unreachable", present.Warn, "-"
	case h.StationVersion != "" && h.StationVersion != hereVersion:
		mark, resources = present.MarkDegraded, "station "+h.StationVersion+" here "+hereVersion
	case h.Daemon.Known && !h.Daemon.Running:
		mark, resources = present.MarkDegraded, "daemon not running · station setup"
	}
	return present.Row{Mark: mark, Cells: []present.Cell{
		{Text: h.Name, Tone: present.Strong},
		{Text: string(catalog.Host), Tone: present.Muted},
		{},
		{},
		{Text: state, Tone: tone},
		{Text: orDash(h.Node.IP), Tone: present.Muted},
		{Text: resources, Tone: present.Muted},
	}}
}

func vmRow(vm machines.VM) present.Row {
	mark, tone := present.MarkOK, present.Good
	resources := resourcesOf(vm)
	switch vm.State {
	case machines.Running:
	case machines.Stopped:
		mark, tone = present.MarkAbsent, present.Muted
	case machines.Suspended:
		mark, tone = present.MarkDegraded, present.Warn
	case machines.Stale:
		mark, tone, resources = present.MarkBroken, present.Bad, "station machines delete "+vm.Name
	case machines.Broken:
		mark, tone, resources = present.MarkBroken, present.Bad, vm.Detail
	}
	return present.Row{Mark: mark, Cells: []present.Cell{
		{Text: vm.Name, Tone: present.Strong},
		{Text: string(vm.Kind), Tone: present.Muted},
		{Text: vm.Host, Tone: present.Muted},
		{Text: string(vm.Lifetime), Tone: present.Muted},
		{Text: string(vm.State), Tone: tone},
		{Text: orDash(vm.IP), Tone: present.Muted},
		{Text: resources, Tone: present.Muted},
	}}
}

func resourcesOf(vm machines.VM) string {
	r := vm.Resources
	return plural(r.CPUs, "cpu") + " · " + memory(r.MemoryBytes) + " · " + present.Bytes(r.DiskBytes)
}

// invariant: memory reads in binary gigabytes, as macOS and tart count RAM, so the
// 8 GB a VM was given prints as 8 GB.
func memory(n int64) string {
	const gib = 1 << 30
	if n%gib == 0 {
		return strconv.FormatInt(n/gib, 10) + " GB"
	}
	return strconv.FormatFloat(float64(n)/gib, 'f', 1, 64) + " GB"
}

func plural(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	return strconv.Itoa(n) + " " + noun + "s"
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
