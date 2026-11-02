package fleetscreen

import (
	"context"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/domain/images"
	"github.com/jterrazz/studio-station/src/domain/machines"
	"github.com/jterrazz/studio-station/src/domain/secrets"
	"github.com/jterrazz/studio-station/src/domain/services"
	"github.com/jterrazz/studio-station/src/domain/tailnet"
	"github.com/jterrazz/studio-station/src/present"
	presentimages "github.com/jterrazz/studio-station/src/present/images"
	presentmachines "github.com/jterrazz/studio-station/src/present/machines"
	presentsecrets "github.com/jterrazz/studio-station/src/present/secrets"
	presentservices "github.com/jterrazz/studio-station/src/present/services"
	presenttailnet "github.com/jterrazz/studio-station/src/present/tailnet"
	"github.com/jterrazz/studio-station/src/theme"
	"github.com/jterrazz/studio-station/src/tui/kit"
)

const ID = "fleet"

type Machines interface {
	Fleet(ctx context.Context, hereOnly bool) (machines.Fleet, error)
	SSHBlocks(ctx context.Context, f machines.Fleet) error
	CheckCreate(ctx context.Context, r machines.CreateRequest) (machines.CreateRequest, error)
	Create(ctx context.Context, r machines.CreateRequest, progress func(machines.Step)) (machines.Created, error)
	Start(ctx context.Context, name string) error
	Stop(ctx context.Context, name string) error
	Delete(ctx context.Context, name string) (machines.DeleteReport, error)
	ForgetHost(name string) error
	Probe(ctx context.Context, name string) (machines.ProbeReport, error)
	Restart(ctx context.Context, name string, progress func(machines.RestartEvent)) (machines.RestartReport, error)
	Unlock(ctx context.Context, name string) error
}

type Images interface {
	List(ctx context.Context) ([]images.Report, error)
	Build(ctx context.Context, r images.BuildRequest, progress func(images.Step)) ([]images.Record, error)
}

type Secrets interface {
	List(ctx context.Context) (secrets.Report, error)
}

type Services interface {
	List(ctx context.Context, hereOnly bool) (services.Fleet, error)
	Release(ctx context.Context, t services.Target, host string, yes bool) (services.Lease, error)
}

type Tailnet interface {
	Status(ctx context.Context) (tailnet.Status, error)
}

type Remote func(ctx context.Context, machine string, words []string) error

type Deps struct {
	Machines Machines
	Images   Images
	Secrets  Secrets
	Services Services
	Tailnet  Tailnet
	Remote   Remote
	Describe kit.Describe
}

type Machine struct {
	Name, Host string
	IsHost     bool
	Here       bool
	Reachable  bool
	Lifetime   machines.Lifetime
	State      machines.VMState
}

type Model struct {
	ctx  context.Context
	d    Deps
	list kit.ListScreen
}

func New(ctx context.Context, d Deps, th theme.Theme) Model {
	m := Model{ctx: ctx, d: d}
	m.list = kit.NewList(kit.ListConfig{
		ID:        ID,
		Load:      m.load,
		Describe:  d.Describe,
		Empty:     "no machines registered",
		EmptyHint: "press n to create a VM",
		Keys: []key.Binding{
			kit.Bind("n", "create", "n"),
			kit.Bind("enter", "open", "enter"),
			kit.Bind("s", "start/stop", "s"),
			kit.Bind("d", "delete", "d"),
			kit.Bind("p", "probe", "p"),
			kit.Bind("f", "forget", "f"),
			kit.Bind("ctrl+r", "restart", "ctrl+r"),
			kit.Bind("o", "unlock", "o"),
			kit.Bind("b", "build images", "b"),
			kit.Bind("x", "release", "x"),
			kit.FilterKey(),
		},
	}, th)
	return m
}

func (m Model) load() ([]kit.Group, kit.Status, error) {
	fleet, err := m.d.Machines.Fleet(m.ctx, false)
	if err != nil {
		return nil, kit.Status{}, err
	}
	var note kit.Status
	if refreshErr := m.d.Machines.SSHBlocks(m.ctx, fleet); refreshErr != nil {
		note = kit.Status{Mark: present.MarkDegraded, Text: "ssh config not refreshed: " + refreshErr.Error()}
	}
	groups := append(hostGroups(fleet), m.servicesGroups()...)
	reports, err := m.d.Images.List(m.ctx)
	groups = append(groups, m.section("Images", err, func() present.Document { return presentimages.List(reports, "").Doc }))
	lent, err := m.d.Secrets.List(m.ctx)
	groups = append(groups, m.section("Secrets", err, func() present.Document { return presentsecrets.List(lent, "").Doc }))
	status, err := m.d.Tailnet.Status(m.ctx)
	tail := m.section("Tailnet", err, func() present.Document {
		return present.Document{Blocks: []present.Block{present.Table{Rows: presenttailnet.Rows(status)}}}
	})
	for i := range tail.Rows {
		tail.Rows[i].Data = status
	}
	return append(groups, tail), note, nil
}

func hostGroups(f machines.Fleet) []kit.Group {
	var table present.Table
	for _, b := range presentmachines.Fleet(f, nil, "").Doc.Blocks {
		if t, ok := b.(present.Table); ok {
			table = t
		}
	}
	next := 0
	groups := make([]kit.Group, 0, len(f.Hosts))
	for _, h := range f.Hosts {
		g := kit.Group{Title: h.Name}
		g.Rows = append(g.Rows, kit.Row{Key: h.Name, Row: table.Rows[next], Data: Machine{
			Name: h.Name, Host: h.Name, IsHost: true, Here: h.Here, Reachable: h.Reachable,
		}})
		next++
		for _, vm := range h.VMs {
			g.Rows = append(g.Rows, kit.Row{Key: vm.Name, Row: table.Rows[next], Data: Machine{
				Name: vm.Name, Host: h.Name, Reachable: h.Reachable, Lifetime: vm.Lifetime, State: vm.State,
			}})
			next++
		}
		g.Counts = counts(g.Rows)
		groups = append(groups, g)
	}
	return groups
}

type Service struct {
	Target services.Target
	Host   string
	Pinned bool
}

// invariant: the Services sections are the fleet's, as `station services` shows
// them: one per host, titled with its range, and a service of another host is
// released on that host.
func (m Model) servicesGroups() []kit.Group {
	f, err := m.d.Services.List(m.ctx, false)
	if err != nil {
		return []kit.Group{m.section("Services", err, nil)}
	}
	var found []Service
	for _, h := range f.Hosts {
		host := h.Name
		if h.Here {
			host = ""
		}
		for _, s := range h.Services {
			found = append(found, Service{Target: services.Target{VM: s.VM, Port: s.GuestPort}, Host: host, Pinned: s.Pinned})
		}
	}
	var groups []kit.Group
	for _, b := range presentservices.Fleet(f, "").Doc.Blocks {
		switch v := b.(type) {
		case present.Section:
			g := kit.Group{Title: "Services · " + v.Title}
			for _, inner := range v.Blocks {
				if t, ok := inner.(present.Table); ok {
					g.Rows = append(g.Rows, rowsOf(t)...)
				}
			}
			groups = append(groups, g)
		case present.Table:
			if len(groups) == 0 {
				groups = append(groups, kit.Group{Title: "Services"})
			}
			last := &groups[len(groups)-1]
			last.Rows = append(last.Rows, rowsOf(v)...)
		}
	}
	for gi := range groups {
		g := &groups[gi]
		for i := range g.Rows {
			cells := g.Rows[i].Row.Cells
			if len(cells) < 2 || len(found) == 0 || cells[0].Text != found[0].Target.VM || cells[1].Text != strconv.Itoa(found[0].Target.Port) {
				continue
			}
			g.Rows[i].Data, found = found[0], found[1:]
		}
		g.Counts = counts(g.Rows)
	}
	return groups
}

func rowsOf(t present.Table) []kit.Row {
	rows := make([]kit.Row, 0, len(t.Rows))
	for _, r := range t.Rows {
		rows = append(rows, kit.Row{Row: r})
	}
	return rows
}

func counts(rows []kit.Row) []present.Count {
	n := map[present.Mark]int{}
	for _, r := range rows {
		n[r.Row.Mark]++
	}
	var out []present.Count
	for _, mark := range []present.Mark{present.MarkOK, present.MarkDegraded, present.MarkAbsent, present.MarkBroken} {
		if n[mark] > 0 {
			out = append(out, present.Count{Mark: mark, N: n[mark]})
		}
	}
	return out
}

// value: Images, Secrets and Tailnet are read beside the fleet; one that cannot
// be read here (a VM keeps no images) says why in its section, never blanks the tab.
func (m Model) section(title string, err error, doc func() present.Document) kit.Group {
	g := kit.Group{Title: title}
	if err != nil {
		text, _ := m.d.Describe(err)
		g.Rows = []kit.Row{{Row: present.Row{Cells: []present.Cell{{Text: text, Tone: present.Muted}}}}}
		return g
	}
	d := doc()
	for _, sub := range kit.GroupsOf(d) {
		g.Rows = append(g.Rows, sub.Rows...)
	}
	for _, b := range d.Blocks {
		if n, ok := b.(present.Notice); ok {
			g.Rows = append(g.Rows, kit.Row{Row: present.Row{Cells: []present.Cell{{Text: n.Text, Tone: present.Muted}}}})
		}
	}
	for i := range g.Rows {
		if title != "Tailnet" {
			g.Rows[i].Key = ""
		}
	}
	g.Counts = counts(g.Rows)
	return g
}

func (m Model) Init() tea.Cmd { return m.list.Init() }

func (m Model) Focus() kit.Focus { return m.list.Focus() }

func (m Model) Busy() bool { return m.list.Busy() }

func (m Model) Loading() bool { return m.list.Loading() }

func (m Model) Keys() []key.Binding { return m.list.Keys() }

func (m Model) View(width, height int) string { return m.list.View(width, height) }

func (m Model) Update(msg tea.Msg) (kit.Screen, tea.Cmd) {
	if k, ok := msg.(tea.KeyPressMsg); ok && m.list.Focus() == kit.FocusNormal {
		if cmd, handled := m.action(k.String()); handled {
			return m, cmd
		}
	}
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func hint(text string) tea.Cmd { return kit.Say(kit.Status{Text: text}) }

func (m *Model) action(pressed string) (tea.Cmd, bool) {
	switch pressed {
	case "n", "b":
		if m.list.Busy() || !m.list.Loaded() {
			return nil, true
		}
		return m.fleetAction(pressed), true
	case "x":
		row, _ := m.list.Current()
		svc, ok := row.Data.(Service)
		if !ok {
			return hint("release: on a service row"), true
		}
		if m.list.Busy() {
			return nil, true
		}
		return m.release(svc), true
	case "s", "d", "f", "p", "ctrl+r", "o", "enter":
	default:
		return nil, false
	}
	row, _ := m.list.Current()
	machine, ok := row.Data.(Machine)
	if !ok {
		return nil, pressed != "enter"
	}
	if m.list.Busy() {
		return nil, true
	}
	return m.machineAction(pressed, machine), true
}

func (m *Model) fleetAction(pressed string) tea.Cmd {
	var cmd tea.Cmd
	switch pressed {
	case "n":
		m.list, cmd = m.list.Open(kit.Form("Create a VM", m.createInputs(), nil, m.create))
	case "b":
		m.list, cmd = m.list.Act(m.exec(func() (kit.Status, error) {
			records, err := m.d.Images.Build(m.ctx, images.BuildRequest{}, nil)
			if err != nil {
				return kit.Status{}, err
			}
			names := make([]string, 0, len(records))
			for _, r := range records {
				names = append(names, r.Name+" "+r.Version)
			}
			return kit.Outcome("built", strings.Join(names, ", "), catalog.StateOK, ""), nil
		}))
	}
	return cmd
}

func (m *Model) machineAction(pressed string, machine Machine) tea.Cmd {
	name := machine.Name
	var cmd tea.Cmd
	switch {
	case pressed == "s" && machine.IsHost:
		return hint("a host: restart it with ctrl+r")
	case pressed == "s" && machine.Lifetime == machines.Temporary:
		return hint("temporary: delete with d")
	case pressed == "s" && machine.State == machines.Running:
		m.list, cmd = m.list.Act(m.exec(func() (kit.Status, error) {
			return kit.Outcome("stopped", name, catalog.StateOK, ""), m.d.Machines.Stop(m.ctx, name)
		}))
	case pressed == "s":
		m.list, cmd = m.list.Act(m.exec(func() (kit.Status, error) {
			return kit.Outcome("started", name, catalog.StateOK, ""), m.d.Machines.Start(m.ctx, name)
		}))
	case pressed == "d" && machine.IsHost:
		return hint("forget: f")
	case pressed == "d":
		m.list, cmd = m.list.Open(kit.Confirm("Delete "+name+"? Everything it holds is destroyed.", m.exec(func() (kit.Status, error) {
			report, err := m.d.Machines.Delete(m.ctx, name)
			if len(report.Warnings) > 0 {
				return kit.Outcome("deleted", name, catalog.StateDegraded, report.Warnings[0]), err
			}
			return kit.Outcome("deleted", name, catalog.StateOK, ""), err
		})))
	case pressed == "f" && !machine.IsHost:
		return hint("delete: d")
	case pressed == "f":
		m.list, cmd = m.list.Open(kit.Confirm("Forget "+name+"? The host itself is not touched.", m.exec(func() (kit.Status, error) {
			return kit.Outcome("forgot", name, catalog.StateOK, ""), m.d.Machines.ForgetHost(name)
		})))
	case pressed == "p":
		m.list, cmd = m.list.Read(name, m.probe(name))
	case pressed == "ctrl+r":
		m.list, cmd = m.list.Open(kit.TypeToConfirm("Restart "+name+"?", name, m.exec(func() (kit.Status, error) {
			_, err := m.d.Machines.Restart(m.ctx, name, nil)
			return kit.Outcome("restarted", name, catalog.StateOK, ""), err
		})))
	case pressed == "o":
		m.list, cmd = m.list.Act(m.exec(func() (kit.Status, error) {
			return kit.Outcome("unlocked", name, catalog.StateOK, ""), m.d.Machines.Unlock(m.ctx, name)
		}))
	case pressed == "enter" && machine.Here:
		return hint("here: this is the machine the TUI runs on")
	case pressed == "enter":
		m.list, cmd = m.list.Act(m.exec(func() (kit.Status, error) {
			return kit.Outcome("back from", name, catalog.StateOK, ""), m.d.Remote(m.ctx, name, nil)
		}))
	}
	return cmd
}

func (m *Model) release(svc Service) tea.Cmd {
	name := svc.Target.VM + ":" + strconv.Itoa(svc.Target.Port)
	run := m.exec(func() (kit.Status, error) {
		_, err := m.d.Services.Release(m.ctx, svc.Target, svc.Host, true)
		return kit.Outcome("released", name, catalog.StateOK, ""), err
	})
	var cmd tea.Cmd
	if svc.Pinned {
		m.list, cmd = m.list.Open(kit.Confirm("Release "+name+"? Its URL is in the VM's environment.", run))
	} else {
		m.list, cmd = m.list.Act(run)
	}
	return cmd
}

func (m Model) exec(action func() (kit.Status, error)) tea.Cmd { return kit.Exec(ID, action) }

func (m Model) probe(name string) tea.Cmd {
	return func() tea.Msg {
		report, err := m.d.Machines.Probe(m.ctx, name)
		return kit.ReadDone{Screen: ID, Title: name, Doc: presentmachines.Probe(report, "").Doc, Err: err}
	}
}

func (m Model) hosts() (here string, reachable []string) {
	for _, r := range m.list.Rows() {
		if machine, ok := r.Data.(Machine); ok && machine.IsHost && machine.Reachable {
			if machine.Here {
				here = machine.Name
				reachable = append([]string{machine.Name}, reachable...)
				continue
			}
			reachable = append(reachable, machine.Name)
		}
	}
	return here, reachable
}

func (m Model) createInputs() []catalog.Input {
	names := make([]string, 0, len(catalog.Images))
	for _, img := range catalog.Images {
		names = append(names, img.Name)
	}
	inputs := []catalog.Input{
		{Name: "lifetime", Label: "Lifetime", Help: "temporary ends with a poweroff inside it; lasting stays until deleted", Kind: catalog.InputSelect, Options: []string{string(machines.Temporary), string(machines.Lasting)}, Required: true},
	}
	if _, hosts := m.hosts(); len(hosts) > 1 {
		inputs = append(inputs, catalog.Input{Name: "host", Label: "On which host?", Kind: catalog.InputSelect, Options: hosts, Required: true})
	}
	return append(inputs,
		catalog.Input{Name: "image", Label: "Image", Kind: catalog.InputSelect, Options: names, Required: true},
		catalog.Input{Name: "name", Label: "Name", Help: "empty: <image>-<n>", Kind: catalog.InputText},
	)
}

func (m Model) create(values map[string]string) tea.Cmd {
	here, _ := m.hosts()
	host := values["host"]
	if host != "" && host != here {
		words := []string{"machines", "create", "--lifetime", values["lifetime"], "--image", values["image"]}
		if values["name"] != "" {
			words = append(words, "--name", values["name"])
		}
		return m.exec(func() (kit.Status, error) {
			return kit.Outcome("created a VM on", host, catalog.StateOK, ""), m.d.Remote(m.ctx, host, words)
		})
	}
	request := machines.CreateRequest{Image: values["image"], Name: values["name"], Lifetime: machines.Lifetime(values["lifetime"])}
	return m.exec(func() (kit.Status, error) {
		resources, err := machines.ParseResources(0, "", "")
		if err != nil {
			return kit.Status{}, err
		}
		request.Resources = resources
		checked, err := m.d.Machines.CheckCreate(m.ctx, request)
		if err != nil {
			return kit.Status{}, err
		}
		created, err := m.d.Machines.Create(m.ctx, checked, nil)
		return kit.Outcome("created", created.VM.Name, catalog.StateOK, "on "+created.VM.Host), err
	})
}
