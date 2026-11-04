package machines

import (
	"strconv"
	"strings"

	"github.com/jterrazz/studio-station/src/domain/machines"
	"github.com/jterrazz/studio-station/src/page"
)

type VMOverview struct {
	Fleet machines.Fleet
	Mac   machines.HostView
	VM    machines.VM
}

func (o VMOverview) on() string {
	if o.Mac.Here {
		return "This Mac"
	}
	return machines.Label(o.Fleet, o.Mac)
}

func (o VMOverview) unreachable() bool { return !o.Mac.Reachable }

func VMPage(o VMOverview, here page.Context) page.Page {
	vm := o.VM
	if o.unreachable() {
		row := page.Row{ID: vm.Name, Label: vm.Name, Mark: page.Broken, Reason: page.Unreachable, Cells: []page.Value{page.Text("is on " + o.on() + ", which does not answer")}}
		return page.Seal(page.Page{ID: "overview", Command: "overview", Context: here, Sections: []page.Section{{
			ID: "overview", Title: vm.Name, Role: page.Banner, Uncounted: true, Columns: vmColumns(), Items: []page.Item{{Row: &row}},
		}}})
	}
	caption := []page.Value{page.Text("Workspace on " + o.on()), page.Text(titled(string(vm.Lifetime))), page.Text(stateWord(vm.State))}
	health := machines.HealthOf(vm)
	steps := []page.Row{
		step("running", machines.StepRunning, health.Running, health.Running, ""),
		step("address", machines.StepAddress, health.Address, health.Running, vm.IP),
		step("ssh", machines.StepSSH, health.SSH, health.Running, ""),
	}
	items := make([]page.Item, 0, len(steps)+1)
	for i := range steps {
		items = append(items, page.Item{Row: &steps[i]})
	}
	r := vm.Resources
	size := page.Row{ID: "size", Label: "Size", Mark: page.OK, Cells: []page.Value{page.List(page.Text(strconv.Itoa(r.CPUs)+" CPU"), Memory(r.MemoryBytes), DiskSize(r.DiskBytes))}}
	items = append(items, page.Item{Row: &size})
	return page.Seal(page.Page{ID: "overview", Command: "overview", Context: here, Sections: []page.Section{{
		ID: "overview", Title: vm.Name, Caption: caption, Role: page.Banner, Uncounted: true, Columns: vmColumns(), Items: items,
	}}})
}

func vmColumns() []page.Column {
	return []page.Column{{Key: "name", Kind: page.KindText, Keep: 100}, {Key: "detail", Kind: page.KindList, Keep: 50}}
}

// value: a step that did not pass fails while the VM runs; one of a VM
// that does not run is not taken.
func step(id, label string, passed, running bool, detail string) page.Row {
	row := page.Row{ID: id, Label: label, Mark: page.OK, Cells: []page.Value{page.None()}}
	if detail != "" {
		row.Cells[0] = page.Text(detail)
	}
	switch {
	case passed:
	case running:
		row.Mark, row.Reason, row.Cells[0] = page.Broken, page.Failing, page.Text("failing")
	default:
		row.Mark, row.Reason = page.Absent, page.Stopped
	}
	return row
}

func titled(word string) string {
	if word == "" {
		return word
	}
	return strings.ToUpper(word[:1]) + word[1:]
}

func stateWord(s machines.VMState) string {
	switch s {
	case machines.Running:
		return "Running"
	case machines.Stopped:
		return "Stopped"
	case machines.Suspended:
		return "Suspended"
	case machines.Stale:
		return "Ended unexpectedly"
	case machines.Broken:
		return "In a loop"
	case machines.Unknown:
	}
	return "Unknown"
}

func DiskSize(n int64) page.Value {
	if n%1e9 == 0 {
		return page.Text(strconv.FormatInt(n/1e9, 10) + " GB")
	}
	return page.Text(strconv.FormatFloat(float64(n)/1e9, 'f', 1, 64) + " GB")
}

type VMHealthJSON struct {
	Running bool `json:"running"`
	Address bool `json:"address"`
	SSH     bool `json:"ssh"`
	// value: a server's check (SPEC §7.2); a workspace has none.
	Check *VMCheckJSON `json:"check"`
}

type VMCheckJSON struct {
	URL    string `json:"url"`
	OK     bool   `json:"ok"`
	Status int    `json:"status"`
	MS     int    `json:"ms"`
	At     string `json:"at"`
}

type VMMachineJSON struct {
	Name      string `json:"name"`
	Kind      string `json:"kind"`
	On        string `json:"on"`
	Reachable bool   `json:"reachable"`
}

type VMOverviewJSON struct {
	Schema   string            `json:"schema"`
	Machine  VMMachineJSON     `json:"machine"`
	Lifetime string            `json:"lifetime"`
	State    string            `json:"state"`
	Wanted   string            `json:"wanted"`
	Health   VMHealthJSON      `json:"health"`
	Restarts int               `json:"restarts"`
	Size     machines.ListSize `json:"size"`
	Detail   string            `json:"detail,omitempty"`
}

func VMContract(o VMOverview, schema string) VMOverviewJSON {
	vm, h := o.VM, machines.HealthOf(o.VM)
	return VMOverviewJSON{
		Schema: schema, Machine: VMMachineJSON{Name: vm.Name, Kind: "workspace", On: o.Mac.Name, Reachable: o.Mac.Reachable},
		Lifetime: string(vm.Lifetime), State: string(vm.State), Wanted: machines.Wanted(vm),
		Health: VMHealthJSON{Running: h.Running, Address: h.Address, SSH: h.SSH},
		Size:   machines.ListSize{CPUs: vm.Resources.CPUs, MemoryBytes: vm.Resources.MemoryBytes, DiskBytes: vm.Resources.DiskBytes},
		Detail: vm.Detail,
	}
}
