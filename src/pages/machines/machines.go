package machines

import (
	"strconv"
	"strings"

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
	// invariant: the pipe keeps SPEC §10.8's eight fields (kind, host, lifetime,
	// and a fix riding in the resources field); the eye reads one class column (a
	// host's `host`, a VM's lifetime: the tree says the rest), the host's role,
	// the resources a short line drops first, and the detail that says why a row
	// is not ● (§13.5).
	return []page.Column{
		{Key: "name", Kind: page.KindText, Keep: 100},
		{Key: "kind", Kind: page.KindText, Show: page.PipeOnly},
		{Key: "host", Kind: page.KindText, Show: page.PipeOnly},
		{Key: "lifetime", Kind: page.KindText, Show: page.PipeOnly},
		{Key: "class", Kind: page.KindText, Keep: 70, Show: page.EyeOnly},
		{Key: "role", Kind: page.KindText, Keep: 30, Show: page.EyeOnly},
		{Key: "state", Kind: page.KindText, Keep: 90},
		{Key: "ip", Kind: page.KindIdentity, Keep: 50},
		{Key: "record", Kind: page.KindList, Show: page.PipeOnly},
		{Key: "resources", Kind: page.KindList, Keep: 10, Whole: true, Show: page.EyeOnly},
		{Key: "detail", Kind: page.KindList, Keep: 5, Show: page.EyeOnly},
	}
}

func Section(f machines.Fleet, links func(host, vm string) []page.Link) page.Section {
	// invariant: one column schema for host and VM rows, and no piped row ends on
	// an empty cell nor grows a hint field, so every record has the same eight
	// fields: a fix rides in the resources cell.
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
	state, resources, detail := page.Text("online"), vmCount(len(h.VMs)), page.None()
	switch {
	case !h.Reachable:
		row.Mark, row.Reason, state, resources = page.Attention, page.Unreachable, page.Text("unreachable"), page.Text("-")
	case h.StationVersion != "" && h.StationVersion != hereVersion:
		row.Mark, row.Reason = page.Attention, page.Mismatch
		detail = page.Text("station " + h.StationVersion + " here " + hereVersion)
	case h.Daemon.Known && !h.Daemon.Running:
		row.Mark, row.Reason = page.Attention, page.Stopped
		word, fix := "engine stopped", "station engine start"
		switch {
		case !h.Here:
			fix = "ssh " + h.Name + " station engine start"
		case !h.Daemon.Installed:
			row.Reason, word, fix = page.Missing, "engine not installed", "station setup"
		}
		detail = page.List(page.Text(word), page.Command(fix))
	}
	role := ""
	if h.Here {
		role = "here"
	}
	row.Cells = []page.Value{page.Text("mac"), page.Text(""), page.Text(""), page.Text("Mac"), page.Text(role), state, orDash(h.Node.IP), record(resources, detail), resources, detail}
	return row
}

func vmRow(vm machines.VM) page.Row {
	row := page.Row{ID: vm.Name, Label: vm.Name, Mark: page.OK}
	resources, detail := Resources(vm), page.None()
	switch vm.State {
	case machines.Running:
	case machines.Stopped:
		row.Mark = page.Absent
		if vm.Detail != "" {
			detail = page.Text(vm.Detail)
		}
	case machines.Suspended:
		row.Mark, row.Reason = page.Attention, page.Stopped
	case machines.Stale:
		row.Mark, row.Reason = page.Attention, page.Outlived
		detail = page.List(page.Text("ended unexpectedly"), page.Text("start or delete it"))
	case machines.Broken:
		row.Mark, row.Reason, detail = page.Broken, page.Failing, page.Text(vm.Detail)
	case machines.Unknown:
		row.Mark, row.Reason = page.NotKnown, page.Unreachable
	}
	row.Cells = []page.Value{page.Text("workspace"), page.Text(vm.Host), page.Text(string(vm.Lifetime)), page.Text(string(vm.Lifetime)), page.Text(""), page.Text(string(vm.State)), orDash(vm.IP), record(resources, detail), resources, detail}
	return row
}

func Resources(vm machines.VM) page.Value {
	r := vm.Resources
	return page.List(plural(r.CPUs, "cpu"), Memory(r.MemoryBytes), page.Size(r.DiskBytes))
}

func Memory(n int64) page.Value {
	// invariant: memory reads in binary gigabytes, as macOS and tart count RAM, so the
	// 8 GB a VM was given prints as 8 GB.
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

// value: the pipe's last field: the fix when the row has one, else its resources.
func record(resources, detail page.Value) page.Value {
	if !detail.IsNone() {
		return detail
	}
	return resources
}

func orDash(s string) page.Value {
	if s == "" {
		return page.Text("-")
	}
	return page.Identity(s)
}

func Change(verb, name, change string, here page.Context) page.Page {
	return pages.Outcome("machines "+verb, here, nil, []page.Row{pages.Done(name, catalog.StateOK, change)}, "machines")
}

func Already(verb, name string, running bool, here page.Context) page.Page {
	// value: a start or stop that found the VM already so: its mark is the state
	// it is in, running or stopped, and nothing changed.
	state, said := catalog.StateAbsent, "is already stopped"
	if running {
		state, said = catalog.StateOK, "is already running"
	}
	return pages.Outcome("machines "+verb, here, nil, []page.Row{pages.Done(name, state, said)}, "machines")
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

func Created(c machines.Created, here page.Context) page.Page {
	vm, r := c.VM, c.VM.Resources
	rows := []page.Row{passed(machines.StepRunning, ""), passed(machines.StepAddress, vm.IP), passed(machines.StepSSH, "")}
	off := false
	for _, s := range c.Services {
		row := pages.Done(s.Profile+"/"+strconv.Itoa(s.GuestPort), catalog.StateOK, "")
		row.Label = s.Profile
		row.Cells = []page.Value{page.Port(int64(s.GuestPort)), page.URL(s.URL)}
		rows = append(rows, row)
		off = off || s.ServeOff
	}
	if off {
		row := page.Row{ID: "ports", Label: "ports", Mark: page.Attention, Reason: page.Degraded, Cells: []page.Value{page.Text(catalog.ServeOffDetail)}}
		rows = append(rows, row)
	}
	head := pages.Done(vm.Name, catalog.StateOK, "")
	head.Cells = []page.Value{page.Text("Created"), page.List(page.Text(titled(string(vm.Lifetime))), page.Text(strconv.Itoa(r.CPUs)+" CPU"), Memory(r.MemoryBytes), DiskSize(r.DiskBytes))}
	rows = append(rows, head)
	columns := []page.Column{
		{Key: "name", Kind: page.KindText, Keep: 100}, {Key: "what", Kind: page.KindText, Keep: 60},
		{Key: "detail", Kind: page.KindList, Keep: 20},
	}
	p := pages.Outcome("machines create", here, columns, rows, "VMs")
	p.Hints = []page.Hint{*page.Cmd("ssh " + vm.Name)}
	return p
}

func passed(label, detail string) page.Row {
	row := pages.Done(label, catalog.StateOK, "")
	row.Cells = []page.Value{page.None(), page.None()}
	if detail != "" {
		row.Cells[0] = page.Text(detail)
	}
	return row
}

func Deleted(r machines.DeleteReport, here page.Context) page.Page {
	rows := make([]page.Row, 0, 1+len(r.Warnings))
	rows = append(rows, pages.Done(r.Name, catalog.StateOK, "deleted"))
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

func CreateForm(lifetime string, hosts []HostChoice) page.Form {
	var f page.Form
	if lifetime == "" {
		f.Fields = append(f.Fields, page.Field{Key: "lifetime", Label: "Will it last?", Kind: page.FieldSelect, Options: []page.Option{
			{Value: "throwaway", Label: "Throwaway: deleted when it powers off from inside"},
			{Value: "permanent", Label: "Permanent: kept until you delete it"},
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

type SSHJSON struct {
	Schema  string `json:"schema"`
	Name    string `json:"name"`
	User    string `json:"user"`
	Host    string `json:"host"`
	Command string `json:"command"`
}

func SSHContract(name string, t machines.SSHTarget) SSHJSON {
	return SSHJSON{Schema: "station.machines.ssh/v1", Name: name, User: t.User, Host: t.Host, Command: t.Command}
}

func SSH(name string, t machines.SSHTarget, here page.Context) page.Page {
	row := pages.Done(name, catalog.StateOK, "")
	row.Cells = []page.Value{page.Command(t.Command)}
	return pages.Outcome("machines ssh "+name, here, []page.Column{{Key: "name", Kind: page.KindText, Keep: 100}, {Key: "command", Kind: page.KindText, Keep: 90}}, []page.Row{row}, "machines")
}

func Edited(e machines.Edited, here page.Context) page.Page {
	// value: the sizes an edit changed, and when they apply; a running VM is
	// told the restart that applies them now.
	var parts []string
	if n := e.Changed.CPUs; n == 1 {
		parts = append(parts, "1 cpu")
	} else if n != 0 {
		parts = append(parts, strconv.Itoa(n)+" cpus")
	}
	if e.Changed.MemoryBytes != 0 {
		parts = append(parts, Memory(e.Changed.MemoryBytes).Text)
	}
	if e.Changed.DiskBytes != 0 {
		parts = append(parts, strconv.FormatFloat(float64(e.Changed.DiskBytes)/1e9, 'f', 1, 64)+" GB disk")
	}
	said := strings.Join(parts, ", ") + ", applies when " + e.Name + " next starts"
	row := pages.Done(e.Name, catalog.StateOK, said)
	if e.Running {
		row.Detail = page.List(page.Text(said), page.Command("station machines restart "+e.Name))
	}
	return pages.Outcome("machines edit "+e.Name, here, nil, []page.Row{row}, "machines")
}
