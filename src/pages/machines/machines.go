package machines

import (
	"strconv"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/domain/machines"
	"github.com/jterrazz/studio-station/src/page"
	"github.com/jterrazz/studio-station/src/pages"
)

const (
	KeyCreate  = "n"
	KeyDelete  = "d"
	KeyStart   = "s"
	KeyStop    = "S"
	KeyRestart = "ctrl+r"
	KeyEnter   = "e"
)

func Columns() []page.Column {
	return []page.Column{
		{Key: "name", Kind: page.KindText, Keep: 100},
		{Key: "kind", Kind: page.KindText, Keep: 70},
		{Key: "host", Kind: page.KindText, Show: page.PipeOnly},
		{Key: "lifetime", Kind: page.KindText, Keep: 60},
		{Key: "state", Kind: page.KindText, Keep: 90},
		{Key: "ip", Kind: page.KindIdentity, Keep: 50},
		{Key: "resources", Kind: page.KindList, Keep: 10},
	}
}

// Section is the Fleet: a host heads its VMs, the one column schema for both
// levels, and no piped row ends on an empty cell nor grows a hint field, so
// every record has the same eight fields: a fix rides in the resources cell. Links, when given, hang each VM's services under it.
func Section(f machines.Fleet, links func(host, vm string) []page.Link) page.Section {
	hereVersion := ""
	for _, h := range f.Hosts {
		if h.Here {
			hereVersion = h.StationVersion
		}
	}
	items := make([]page.Item, 0, len(f.Hosts))
	for _, h := range f.Hosts {
		g := page.Group{Head: hostRow(h, hereVersion), Fold: page.Open}
		for _, vm := range h.VMs {
			row := vmRow(vm)
			var under []page.Item
			if links != nil {
				for _, l := range links(h.Name, vm.Name) {
					under = append(under, page.Item{Link: &l})
				}
			}
			if len(under) == 0 {
				g.Items = append(g.Items, page.Item{Row: &row})
				continue
			}
			g.Items = append(g.Items, page.Item{Group: &page.Group{Head: row, Items: under, Fold: page.Open}})
		}
		if len(h.VMs) == 0 && h.Reachable {
			hint := page.KeyHint("station machines create", KeyCreate)
			if !h.Here {
				hint = page.Cmd("station machines create --on " + h.Name)
			}
			g.Items = append(g.Items, page.Item{Empty: &page.Empty{Noun: "VMs", Hint: hint}})
		}
		items = append(items, page.Item{Group: &g})
	}
	if len(items) == 0 {
		items = append(items, page.Item{Empty: &page.Empty{Noun: "hosts", Hint: page.Cmd("station machines add <name>")}})
	}
	return page.Section{ID: "fleet", Title: "Fleet", Columns: Columns(), Items: items}
}

func List(f machines.Fleet, warnings []string, here page.Context) page.Page {
	s := Section(f, nil)
	s.Role = page.Banner
	sections := []page.Section{s}
	if len(warnings) > 0 {
		notes := make([]page.Item, 0, len(warnings))
		for _, w := range warnings {
			notes = append(notes, page.Item{Note: &page.Note{Text: w}})
		}
		sections = append(sections, page.Section{ID: "warnings", Title: "Warnings", Role: page.Banner, Items: notes})
	}
	return page.Seal(page.Page{ID: "machines", Command: "machines", Context: here, Sections: sections, Actions: Actions()})
}

func Actions() []page.Action {
	return []page.Action{
		{Key: KeyCreate, Label: "create", On: page.OnPage, Priority: 4, Mutates: true},
		{Key: KeyDelete, Label: "delete", On: page.OnRow, Priority: 1, Mutates: true, Confirm: &page.Confirm{Yes: "delete", Danger: true}},
		{Key: KeyStart, Label: "start", On: page.OnRow, Priority: 5, Mutates: true},
		{Key: KeyStop, Label: "stop", On: page.OnRow, Priority: 6, Mutates: true},
		{Key: KeyRestart, Label: "restart", On: page.OnRow, Priority: 2, Mutates: true, Confirm: &page.Confirm{Yes: "restart", Danger: true}},
		{Key: KeyEnter, Label: "enter", On: page.OnRow, Priority: 3, Mutates: true},
	}
}

func hostRow(h machines.HostView, hereVersion string) page.Row {
	row := page.Row{ID: h.Name, Label: h.Name, Mark: page.OK}
	state, resources := page.Text("online"), vmCount(len(h.VMs))
	switch {
	case !h.Reachable:
		row.Mark, row.Reason, state, resources = page.Attention, page.Unreachable, page.Text("unreachable"), page.Text("-")
	case h.StationVersion != "" && h.StationVersion != hereVersion:
		row.Mark, row.Reason = page.Attention, page.Mismatch
		resources = page.Text("station " + h.StationVersion + " here " + hereVersion)
	case h.Daemon.Known && !h.Daemon.Running:
		row.Mark, row.Reason = page.Attention, page.Stopped
		resources = page.List(page.Text("daemon not running"), page.Command("station setup"))
	}
	row.Cells = []page.Value{page.Text(string(catalog.Host)), page.Text(""), page.Text(""), state, orDash(h.Node.IP), resources}
	return row
}

func vmRow(vm machines.VM) page.Row {
	row := page.Row{ID: vm.Name, Label: vm.Name, Mark: page.OK}
	resources := Resources(vm)
	switch vm.State {
	case machines.Running:
	case machines.Stopped:
		row.Mark = page.Absent
	case machines.Suspended:
		row.Mark, row.Reason = page.Attention, page.Stopped
	case machines.Stale:
		row.Mark, row.Reason = page.Broken, page.Outlived
		resources = page.Command("station machines delete " + vm.Name)
	case machines.Broken:
		row.Mark, row.Reason, resources = page.Broken, page.Failing, page.Text(vm.Detail)
	}
	row.Cells = []page.Value{page.Text(string(vm.Kind)), page.Text(vm.Host), page.Text(string(vm.Lifetime)), page.Text(string(vm.State)), orDash(vm.IP), resources}
	return row
}

func Resources(vm machines.VM) page.Value {
	r := vm.Resources
	return page.List(plural(r.CPUs, "cpu"), Memory(r.MemoryBytes), page.Size(r.DiskBytes))
}

// invariant: memory reads in binary gigabytes, as macOS and tart count RAM, so the
// 8 GB a VM was given prints as 8 GB.
func Memory(n int64) page.Value {
	const gib = 1 << 30
	if n%gib == 0 {
		return page.Text(strconv.FormatInt(n/gib, 10) + " GB")
	}
	return page.Text(strconv.FormatFloat(float64(n)/gib, 'f', 1, 64) + " GB")
}

func plural(n int, noun string) page.Value {
	if n == 1 {
		return page.Count(1, noun)
	}
	return page.Count(int64(n), noun+"s")
}

func vmCount(n int) page.Value { return plural(n, "VM") }

func orDash(s string) page.Value {
	if s == "" {
		return page.Text("-")
	}
	return page.Identity(s)
}

// Change is the outcome of add, forget, start, stop, restart: one row.
func Change(verb, name, change string, here page.Context) page.Page {
	return pages.Outcome("machines "+verb, here, nil, []page.Row{pages.Done(name, catalog.StateOK, change)}, "machines")
}

type CheckJSON struct {
	Name   string `json:"name"`
	State  string `json:"state"`
	Value  string `json:"value"`
	Detail string `json:"detail"`
}

type ProbeJSON struct {
	Schema  string      `json:"schema"`
	Name    string      `json:"name"`
	User    string      `json:"user"`
	IP      string      `json:"ip"`
	Checks  []CheckJSON `json:"checks"`
	Console string      `json:"console"`
}

func ProbeContract(r machines.ProbeReport) ProbeJSON {
	body := ProbeJSON{Schema: "station.machines.probe/v1", Name: r.Name, User: r.User, IP: r.IP, Console: r.Console, Checks: []CheckJSON{}}
	for _, c := range r.Checks {
		body.Checks = append(body.Checks, CheckJSON{Name: c.Name, State: string(c.State), Value: c.Value, Detail: c.Detail})
	}
	return body
}

func Probe(r machines.ProbeReport, here page.Context) page.Page {
	rows := make([]page.Row, 0, len(r.Checks))
	for _, c := range r.Checks {
		row := pages.Done(c.Name, c.State, c.Detail)
		row.Cells = []page.Value{page.Text(c.Value)}
		if row.Mark == page.Attention || row.Mark == page.Broken {
			row.Reason = page.Down
		}
		rows = append(rows, row)
	}
	return pages.Outcome("machines probe "+r.Name, here, []page.Column{{Key: "check", Kind: page.KindText, Keep: 100}, {Key: "value", Kind: page.KindText, Keep: 50}}, rows, "checks")
}

const createdSchema = "station.machines.created/v1"

type CreatedJSON struct {
	Schema      string            `json:"schema"`
	Name        string            `json:"name"`
	Host        string            `json:"host"`
	Lifetime    string            `json:"lifetime"`
	Image       string            `json:"image"`
	Version     string            `json:"version"`
	CPUs        int               `json:"cpus"`
	MemoryBytes int64             `json:"memory_bytes"`
	DiskBytes   int64             `json:"disk_bytes"`
	IP          string            `json:"ip"`
	Network     string            `json:"network"`
	Services    []CreatedService  `json:"services"`
	Env         map[string]string `json:"env"`
}

type CreatedService struct {
	GuestPort int    `json:"guest_port"`
	HostPort  int    `json:"host_port"`
	Profile   string `json:"profile"`
	URL       string `json:"url"`
}

func CreatedContract(c machines.Created) CreatedJSON {
	vm, r := c.VM, c.VM.Resources
	body := CreatedJSON{
		Schema: createdSchema, Name: vm.Name, Host: vm.Host, Lifetime: string(vm.Lifetime), Image: vm.Image,
		Version: vm.Version, CPUs: r.CPUs, MemoryBytes: r.MemoryBytes, DiskBytes: r.DiskBytes, IP: vm.IP,
		Network: vm.Network, Services: []CreatedService{}, Env: c.Env,
	}
	if body.Env == nil {
		body.Env = map[string]string{}
	}
	for _, s := range c.Services {
		body.Services = append(body.Services, CreatedService{GuestPort: s.GuestPort, HostPort: s.HostPort, Profile: s.Profile, URL: s.URL})
	}
	return body
}

// Created: the name is the second field of the piped row, so `cut -f2` names
// the VM a script just made; each profile's URL follows on its own row.
func Created(c machines.Created, here page.Context) page.Page {
	vm, r := c.VM, c.VM.Resources
	head := pages.Done(vm.Name, catalog.StateOK, "")
	head.Cells = []page.Value{page.Text(vm.Host), page.Text(string(vm.Lifetime)), page.Text(vm.Image + " " + vm.Version), page.List(plural(r.CPUs, "cpu"), Memory(r.MemoryBytes))}
	rows := []page.Row{head}
	off := false
	for _, s := range c.Services {
		row := pages.Done(s.Profile+"/"+strconv.Itoa(s.GuestPort), catalog.StateOK, "")
		row.Label = s.Profile
		row.Cells = []page.Value{page.Port(int64(s.GuestPort)), page.URL(s.URL)}
		rows = append(rows, row)
		off = off || s.ServeOff
	}
	if off {
		row := page.Row{ID: "services", Label: "services", Mark: page.Attention, Reason: page.Degraded, Cells: []page.Value{page.Text(catalog.ServeOffDetail)}}
		rows = append(rows, row)
	}
	columns := []page.Column{
		{Key: "name", Kind: page.KindText, Keep: 100}, {Key: "host", Kind: page.KindText, Keep: 40},
		{Key: "lifetime", Kind: page.KindText, Keep: 60}, {Key: "image", Kind: page.KindText, Keep: 30},
		{Key: "resources", Kind: page.KindList, Keep: 20},
	}
	p := pages.Outcome("machines create", here, columns, rows, "VMs")
	p.Hints = []page.Hint{*page.Cmd("ssh " + vm.Name)}
	return p
}

func Deleted(r machines.DeleteReport, here page.Context) page.Page {
	rows := []page.Row{pages.Done(r.Name, catalog.StateOK, "deleted")}
	for i, w := range r.Warnings {
		row := pages.Done(r.Name+"/"+strconv.Itoa(i), catalog.StateDegraded, w)
		row.Label = r.Name
		rows = append(rows, row)
	}
	return pages.Outcome("machines delete "+r.Name, here, nil, rows, "VMs")
}

func Restarted(name string, here page.Context) page.Page {
	return pages.Outcome("machines restart "+name, here, nil, []page.Row{pages.Done(name, catalog.StateOK, "restarted")}, "VMs")
}

type HostChoice struct {
	Name, Detail string
	Here         bool
}

// CreateForm: the lifetime is asked, never defaulted, and offers exactly the
// two lifetimes; the host is a question only when there is a choice, two
// reachable hosts or more, and it defaults to here.
func CreateForm(lifetime string, hosts []HostChoice) page.Form {
	var f page.Form
	if lifetime == "" {
		f.Fields = append(f.Fields, page.Field{Key: "lifetime", Label: "Will it last?", Kind: page.FieldSelect, Options: []page.Option{
			{Value: "temporary", Label: "temporary: a poweroff inside it deletes it"},
			{Value: "lasting", Label: "lasting: it stays until deleted"},
		}})
	}
	if len(hosts) >= 2 {
		field := page.Field{Key: "host", Label: "On which host?", Kind: page.FieldSelect}
		for _, h := range hosts {
			field.Options = append(field.Options, page.Option{Value: h.Name, Label: h.Name, Note: h.Detail})
			if h.Here {
				field.Default = h.Name
			}
		}
		f.Fields = append(f.Fields, field)
	}
	return f
}

func BuildConfirm(image string) page.Confirm {
	return page.Confirm{Question: "Image " + image + " is not built here. Build " + image + " now? (~3 min)"}
}

func Count(n int) page.Value { return vmCount(n) }
