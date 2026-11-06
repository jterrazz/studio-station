package menubar

import (
	"slices"
	"strconv"
	"time"

	"github.com/jterrazz/studio-station/src/accounts"
	"github.com/jterrazz/studio-station/src/activity"
	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/core/machine"

	model "github.com/jterrazz/studio-station/src/machines"
	"github.com/jterrazz/studio-station/src/overview"
	"github.com/jterrazz/studio-station/src/page"
	pagehome "github.com/jterrazz/studio-station/src/pages/home"
	pagemachines "github.com/jterrazz/studio-station/src/pages/machines"
	"github.com/jterrazz/studio-station/src/ports"
)

const (
	MachineID  page.ID = "menubar.machine"
	SwitcherID page.ID = "menubar.switcher"

	ActionStop     = "stop"
	ActionDelete   = "delete"
	ActionRestart  = "restart"
	ActionShare    = "share-port"
	ActionTakeBack = "take-back"
	ActionUnlock   = "unlock"
	ActionAddMac   = "add-mac"

	SectionMachine  = "machine"
	SectionActivity = "activity"
	SectionInside   = "inside"
	SectionPorts    = "ports"
	SectionShared   = "shared"
	SectionUp       = "up"
	SectionHealth   = "health"
	SectionServer   = "server"
	SectionVMs      = "vms"
)

type Machine struct {
	// value: the machine the page is of, a VM or a Mac, by its name among
	// the machines; a nil part is still being read.
	Name     string
	All      *model.AllMachines
	Summary  *overview.Summary
	Samples  []activity.Reading
	Ports    *ports.AllMachines
	Accounts *accounts.Report
	Now      time.Time
	Asks     Levels
}

func MachinePage(m Machine) page.Page {
	// invariant: SPEC §10.2's page of one machine: a workspace's, a server's or
	// a Mac's, each with only the actions its kind takes, each naming its call
	// and asking at the level the call asks.
	if m.All == nil {
		row := page.Row{ID: "reading", Label: m.Name, Mark: page.Awaiting}
		return page.Seal(page.Page{ID: MachineID, Command: "menubar", Context: page.Context{Machine: m.Name}, Sections: []page.Section{{ID: SectionMachine, Title: m.Name, Role: page.Summary, Items: []page.Item{{Row: &row}}}}})
	}
	for _, h := range m.All.Macs {
		if h.Name == m.Name {
			return macPage(m, h)
		}
		for _, vm := range h.VMs {
			if vm.Name == m.Name {
				return vmPage(m, h, vm)
			}
		}
	}
	return page.Seal(page.Page{ID: MachineID, Command: "menubar", Context: page.Context{Machine: m.Name},
		Error: &page.Error{Message: "no machine is named " + m.Name, Hint: "station machines"}})
}

func target(h model.MacView, name string) *page.Target {
	if h.ThisMac {
		return nil
	}
	return &page.Target{Name: name, On: h.Name}
}

func vmPage(m Machine, h model.MacView, vm model.VM) page.Page {
	kind := vm.Lifetime.Kind()
	on := model.Label(*m.All, h)
	head := page.Row{ID: "state", Label: "State", Mark: page.OK, Cells: []page.Value{pagemachines.State(vm.State)}}
	if vm.State != model.Running {
		head.Mark, head.Reason = page.Absent, page.Stopped
	}
	if model.Down(vm) {
		head.Mark, head.Reason = page.Broken, page.Down
	}
	caption := []page.Value{page.Text(model.KindWord(kind) + " VM"), pagemachines.Lifetime(vm.Lifetime), page.Text("on " + on)}
	if kind == machine.Server {
		r := vm.Resources
		caption = append(caption, page.Text(strconv.Itoa(r.CPUs)+" CPU"), pagemachines.Memory(r.MemoryBytes))
	}
	sections := []page.Section{{ID: SectionMachine, Title: vm.Name, Role: page.Summary, Caption: caption, Columns: columns(), Items: []page.Item{{Row: &head}}}}
	p := page.Page{ID: MachineID, Command: "menubar", Context: page.Context{Machine: vm.Name, Kind: string(kind)}}
	if kind == machine.Server {
		sections = append(sections, upSection(m, vm), healthSection(vm), activitySection(m.Samples, "Activity"), serverSection(m))
		restart := page.Action{ID: ActionRestart, Label: "Restart", On: page.OnPage, Priority: 1, Mutates: true, Confirm: ask(m.Asks, "restart", kind, vm.Name, "Restart "+vm.Name+"?", "restart"),
			Run: &page.Run{Call: CallRestartVM, In: map[string]string{"Name": vm.Name}, Target: target(h, vm.Name)}}
		restart.Run.In = answered(restart.Confirm, restart.Run.In)
		stop := page.Action{ID: ActionStop, Label: "Stop", On: page.OnPage, Priority: 2, Mutates: true,
			Confirm: ask(m.Asks, "stop", kind, vm.Name, "Stop "+vm.Name+"?", "stop", "It stays stopped until you start it: the engine no longer restarts it"),
			Run:     &page.Run{Call: CallStopVM, In: map[string]string{"VM": vm.Name}, Target: target(h, vm.Name)}}
		stop.Run.In = answered(stop.Confirm, stop.Run.In)
		p.Actions = []page.Action{restart, stop}
	} else {
		sections = append(sections, activitySection(m.Samples, "Activity"), insideSection(m.Summary), portsSection(m, vm), sharedSection(m, h, vm))
		stop := page.Action{ID: ActionStop, Label: "Stop", On: page.OnPage, Priority: 1, Mutates: true,
			Confirm: ask(m.Asks, "stop", kind, vm.Name, "Stop "+vm.Name+"?", "stop", "Its disk is kept"),
			Run:     &page.Run{Call: CallStopVM, In: map[string]string{"VM": vm.Name}, Target: target(h, vm.Name)}}
		stop.Run.In = answered(stop.Confirm, stop.Run.In)
		del := page.Action{ID: ActionDelete, Label: "Delete", On: page.OnPage, Priority: 2, Mutates: true,
			Confirm: ask(m.Asks, "delete", kind, vm.Name, "Delete "+vm.Name+"?", "delete", "Its disk is erased: nothing of it is kept"),
			Run:     &page.Run{Call: CallDeleteVM, In: map[string]string{"VM": vm.Name}, Target: target(h, vm.Name)}}
		del.Run.In = answered(del.Confirm, del.Run.In)
		share := page.Action{ID: ActionShare, Label: "Share a port", On: page.OnPage, Priority: 3, Mutates: true,
			Run:  &page.Run{Call: CallSharePort, In: map[string]string{"VM": vm.Name}, Target: target(h, vm.Name)},
			Form: &page.Form{Title: "Share a port of " + vm.Name, Fields: []page.Field{{Key: "Port", Label: "Port", Kind: page.FieldText, Required: true, Note: "private, on " + catalog.TailnetProduct}}}}
		p.Actions = append([]page.Action{stop, del, share}, takeBacks(m, vm)...)
	}
	p.Actions = append(p.Actions, openIn("--on", vm.Name))
	p.Sections = sections
	return page.Seal(p)
}

// value: Activity's three measures of the machine, as its last two
// readings give them, each led into its series over the readings kept
// (0 to 100: CPU's share, memory's and disk's use of their total), so a
// tile draws its bar meter; pending until they are read.
func activitySection(readings []activity.Reading, title string) page.Section {
	cpu, memory, disk := page.Pending(), page.Pending(), page.Pending()
	var cpus, memories, disks []int
	for i, r := range readings {
		if i > 0 {
			cpus = append(cpus, int(activity.Between(readings[i-1].Sample, r.Sample).CPU))
		}
		if s := r.Sample; s.MemTotal > 0 {
			memories = append(memories, int(min(s.MemUsed, s.MemTotal)*100/s.MemTotal))
		}
		if s := r.Sample; s.DiskTotal > 0 {
			disks = append(disks, int((s.DiskTotal-s.DiskFree)*100/s.DiskTotal))
		}
	}
	if n := len(readings); n > 0 {
		last := readings[n-1].Sample
		if n > 1 {
			cpu = page.Percent(int64(activity.Between(readings[n-2].Sample, last).CPU * 10))
		}
		memory, disk = page.None(), page.None()
		if last.MemTotal > 0 {
			memory = page.Ratio(min(last.MemUsed, last.MemTotal), last.MemTotal)
		}
		if last.DiskTotal > 0 {
			disk = page.Ratio(last.DiskTotal-last.DiskFree, last.DiskTotal)
		}
	}
	measure := func(id, label string, now page.Value, series []int) page.Row {
		row := page.Row{ID: id, Label: label, Cells: []page.Value{now}}
		if len(series) > 1 {
			row.Cells = append(row.Cells, page.Spark(series, page.None()))
		}
		return row
	}
	rows := []page.Row{measure("cpu", "CPU", cpu, cpus), measure("memory", "Memory", memory, memories), measure("disk", "Disk", disk, disks)}
	cols := append(columns(), page.Column{Key: "graph", Kind: page.KindSpark, Keep: 50})
	return page.Section{ID: SectionActivity, Title: title, Role: page.Summary, Columns: cols, Items: items(rows)}
}

func items(rows []page.Row) []page.Item {
	out := make([]page.Item, 0, len(rows))
	for i := range rows {
		out = append(out, page.Item{Row: &rows[i]})
	}
	return out
}

// value: a workspace's Apps, Settings and Disk, as its Overview counts them
// inside it: station manages them there.
func insideSection(s *overview.Summary) page.Section {
	rows := []page.Row{{ID: "apps", Label: "Apps"}, {ID: "settings", Label: "Settings"}, {ID: "disk", Label: "Disk"}}
	for i := range rows {
		rows[i].Cells = []page.Value{page.Pending()}
		if s == nil {
			continue
		}
		rows[i].Cells = []page.Value{page.None()}
		for _, l := range s.Lines {
			if string(l.Section) == rows[i].ID {
				rows[i].Cells = []page.Value{lineOf(*s, rows[i].ID)}
			}
		}
	}
	return page.Section{ID: SectionInside, Title: "Inside", Role: page.Summary, Columns: columns(), Items: items(rows)}
}

func lineOf(s overview.Summary, id string) page.Value {
	for _, body := range pagehome.Body(s, true) {
		for _, it := range body.Items {
			if body.ID == "sections" && it.Row != nil && it.Row.ID == id && len(it.Row.Cells) > 0 {
				return it.Row.Cells[0]
			}
		}
	}
	return page.None()
}

// value: the workspace's shared ports, each a link: private, on Tailscale.
func portsSection(m Machine, vm model.VM) page.Section {
	s := page.Section{ID: SectionPorts, Title: "Private", Caption: []page.Value{page.Text("shared on " + catalog.TailnetProduct)}, Uncounted: true}
	if m.Ports == nil {
		s.Items = []page.Item{{Row: &page.Row{ID: "reading", Label: "reading", Mark: page.Awaiting}}}
		return s
	}
	for _, h := range m.Ports.Macs {
		for _, sv := range h.Shared {
			if sv.VM != vm.Name {
				continue
			}
			value := page.Text(h.PublicName + ":" + strconv.Itoa(sv.Reserved))
			if sv.URLs.Public != "" {
				value = page.URL(sv.URLs.Public)
			}
			label := cmpWord(titled(sv.Profile), "TCP")
			s.Items = append(s.Items, page.Item{Link: &page.Link{Label: label, Lead: page.List(page.Text(label), page.Port(int64(sv.GuestPort))), Value: value, Row: strconv.Itoa(sv.GuestPort)}})
		}
	}
	if len(s.Items) == 0 {
		s.Items = []page.Item{{Empty: &page.Empty{Text: "No port shared yet"}}}
	}
	return s
}

// value: the accounts This Mac shared with the workspace, each with its time
// left and its Take back.
func sharedSection(m Machine, h model.MacView, vm model.VM) page.Section {
	from := model.Label(*m.All, h)
	s := page.Section{ID: SectionShared, Title: "Accounts shared from " + from, Columns: columns()}
	for _, a := range sharedAccounts(m, vm.Name) {
		left := a.Shared.Until.Sub(m.Now)
		row := page.Row{ID: a.Provider, Label: accounts.Label(a.Provider), Mark: page.OK, Cells: []page.Value{page.List(page.Text(hours(left) + " left"))}}
		s.Items = append(s.Items, page.Item{Row: &row})
	}
	if len(s.Items) == 0 {
		s.Items = []page.Item{{Empty: &page.Empty{Text: "No account shared"}}}
	}
	return s
}

func sharedAccounts(m Machine, vm string) []accounts.Account {
	if m.Accounts == nil {
		return nil
	}
	var out []accounts.Account
	for _, mc := range m.Accounts.Machines {
		if mc.Name != vm {
			continue
		}
		for _, a := range mc.Accounts {
			if a.Shared != nil {
				out = append(out, a)
			}
		}
	}
	return out
}

func hours(d time.Duration) string {
	if d < time.Hour {
		return strconv.Itoa(max(int(d/time.Minute), 1)) + " min"
	}
	return strconv.Itoa(int(d/time.Hour)) + " h"
}

func takeBacks(m Machine, vm model.VM) []page.Action {
	shared := sharedAccounts(m, vm.Name)
	out := make([]page.Action, 0, len(shared))
	for _, a := range shared {
		label := accounts.Label(a.Provider)
		c := ask(m.Asks, "take-back", machine.Workspace, label, "Take back "+label+" from "+vm.Name+"?", "take back", "Its copy on "+vm.Name+" is deleted")
		out = append(out, page.Action{ID: ActionTakeBack + "/" + a.Provider, Label: "Take back", On: page.OnRow, Rows: []string{a.Provider}, Priority: 4, Mutates: true, Confirm: c,
			Run: &page.Run{Call: CallTakeBack, In: answered(c, map[string]string{"Account": a.Provider, "From": vm.Name})}})
	}
	return out
}

func upSection(m Machine, vm model.VM) page.Section {
	up := page.Row{ID: "up", Label: "Up", Cells: []page.Value{page.None()}}
	if m.Summary != nil && m.Summary.Head.Up > 0 {
		up.Cells = []page.Value{pagehome.Up(m.Summary.Head.Up)}
	}
	words := vm.Restarts.Words(m.Now)
	restarts := page.Row{ID: "restarts", Label: "Restarts", Cells: []page.Value{page.List(page.Count(int64(vm.Restarts.Count), plural(vm.Restarts.Count, "restart")), page.Text(words[len(words)-1]))}}
	return page.Section{ID: SectionUp, Title: "Overview", Role: page.Summary, Columns: columns(), Items: items([]page.Row{up, restarts})}
}

func healthSection(vm model.VM) page.Section {
	return page.Section{ID: SectionHealth, Title: "Health", Caption: []page.Value{page.Text("after every boot")}, Columns: columns(), Items: items(pagemachines.Steps(vm))}
}

// value: what a server's page adds, as its Overview reads it: its ports'
// counts and the command that opens a shell on it, for debugging; station
// manages nothing inside it.
func serverSection(m Machine) page.Section {
	s := page.Section{ID: SectionServer, Title: "Ports and shell", Role: page.Summary, Columns: columns()}
	if m.Summary == nil {
		s.Items = []page.Item{{Row: &page.Row{ID: "reading", Label: "reading", Cells: []page.Value{page.Pending()}}}}
		return s
	}
	for _, body := range pagehome.Body(*m.Summary, true) {
		if body.ID == "sections" {
			s.Items = append(s.Items, body.Items...)
		}
	}
	return s
}

// value: a Mac's page: its readiness checklist, its VMs, its Needs you;
// Unlock… while it waits for its disk's password.
func macPage(m Machine, h model.MacView) page.Page {
	label := model.Label(*m.All, h)
	state := page.Row{ID: "state", Label: "State", Mark: page.OK, Cells: []page.Value{page.Text("Engine running")}}
	switch {
	case !h.Reachable && !h.ThisMac:
		state.Mark, state.Reason, state.Cells = page.Broken, page.Unreachable, []page.Value{page.Text("Unreachable")}
	case !h.Engine.Running:
		state.Mark, state.Reason, state.Cells = page.Attention, page.Stopped, []page.Value{page.Text("Engine stopped")}
	}
	sections := []page.Section{{ID: SectionMachine, Title: label, Role: page.Summary, Caption: []page.Value{page.Text(KindMac), page.Text(h.Name)}, Columns: columns(), Items: []page.Item{{Row: &state}}}}
	if m.Summary != nil {
		for _, body := range pagehome.Body(*m.Summary, true) {
			if body.ID == SectionNeeds || body.ID == "readiness" {
				sections = append(sections, body)
			}
		}
	}
	vms := Tree(&model.AllMachines{Macs: []model.MacView{h}}, m.Ports, func(model.MacView) page.Value { return page.None() })
	vms.ID, vms.Title = SectionVMs, "VMs"
	if len(vms.Items) == 1 && vms.Items[0].Group != nil {
		vms.Items = vms.Items[0].Group.Items
	}
	if len(vms.Items) == 0 {
		vms.Items = []page.Item{{Empty: &page.Empty{Noun: "VMs"}}}
	}
	sections = append(sections, vms)
	p := page.Page{ID: MachineID, Command: "menubar", Context: page.Context{Machine: h.Name, Kind: string(machine.Mac)}, Sections: sections}
	if locked(m, h) {
		p.Actions = append(p.Actions, page.Action{ID: ActionUnlock, Label: "Unlock", On: page.OnPage, Priority: 1, Mutates: true, Run: &page.Run{Terminal: true, Argv: []string{"machines", "unlock", label}}})
	}
	if !h.ThisMac {
		p.Actions = append(p.Actions, openIn("--on", label))
	} else {
		p.Actions = append(p.Actions, openIn())
	}
	return page.Seal(p)
}

// value: the Mac waits for its disk's password, as its readiness or its
// unreachable row says: Unlock… is its one action that helps.
func locked(m Machine, h model.MacView) bool {
	if m.Summary != nil && slices.ContainsFunc(m.Summary.Readiness, func(c overview.Check) bool {
		return c.Name == model.ItemDiskUnlock && c.State != catalog.StateOK
	}) {
		return true
	}
	return !h.Reachable && !h.ThisMac && h.LAN != ""
}

func Switcher(f *model.AllMachines, svc *ports.AllMachines) page.Page {
	// invariant: SPEC §10.2's switcher: All machines as a tree, each Mac then its
	// Servers then its Workspaces, and Add a Mac…; Recent is the app's own.
	s := Tree(f, svc, func(h model.MacView) page.Value {
		if h.Engine.Running {
			return page.Text("Engine running")
		}
		return page.Text("Engine stopped")
	})
	s.Title = "All machines"
	add := page.Action{ID: ActionAddMac, Label: "Add a Mac", On: page.OnPage, Priority: 1, Mutates: true, Run: &page.Run{Call: CallAddMac},
		Form: &page.Form{Title: "Add a Mac", Fields: []page.Field{{Key: "Name", Label: "Name", Kind: page.FieldText, Required: true, Note: "as " + catalog.TailnetProduct + " names it"}}}}
	return page.Seal(page.Page{ID: SwitcherID, Command: "menubar", Sections: []page.Section{s}, Actions: []page.Action{add}})
}
