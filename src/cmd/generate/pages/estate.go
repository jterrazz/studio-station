package pages

import (
	"strings"
	"time"

	"github.com/jterrazz/studio-station/src/images"

	"github.com/jterrazz/studio-station/src/apps"

	coremachine "github.com/jterrazz/studio-station/src/core/machine"

	"github.com/jterrazz/studio-station/src/background"
	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/disk"
	"github.com/jterrazz/studio-station/src/domain/machines"
	"github.com/jterrazz/studio-station/src/overview"
	"github.com/jterrazz/studio-station/src/page"
	allpages "github.com/jterrazz/studio-station/src/pages"
	pageapps "github.com/jterrazz/studio-station/src/pages/apps"
	pagedisk "github.com/jterrazz/studio-station/src/pages/disk"
	pagefleet "github.com/jterrazz/studio-station/src/pages/fleet"
	pagehome "github.com/jterrazz/studio-station/src/pages/home"
	pageimages "github.com/jterrazz/studio-station/src/pages/images"
	pagemachines "github.com/jterrazz/studio-station/src/pages/machines"
	pageports "github.com/jterrazz/studio-station/src/pages/ports"
	pagesettings "github.com/jterrazz/studio-station/src/pages/settings"
	pagesetup "github.com/jterrazz/studio-station/src/pages/setup"
	"github.com/jterrazz/studio-station/src/ports"
	"github.com/jterrazz/studio-station/src/settings"
	"github.com/jterrazz/studio-station/src/setup"
)

var (
	built = time.Date(2026, 10, 30, 9, 0, 0, 0, time.UTC)
	gb    = int64(1) << 30
)

// invariant: a page with no loading frame of its own (a one-shot read or an
// outcome) is proved loading as its full form with every row pending, the
// frame a live read would draw before its probes answer.
func states(name string, loading, empty, failed, full page.Page) []Fixture {
	return []Fixture{{name + ".loading", loading}, {name + ".empty", empty}, {name + ".error", failed}, {name + ".full", full}}
}

func pending(p page.Page) page.Page {
	for si := range p.Sections {
		pendAll(p.Sections[si].Items)
	}
	p.Strip = nil
	return page.Seal(p)
}

func failure(id page.ID, command, message, hint string) page.Page {
	return allpages.Failure(id, command, here(), message, hint)
}

func Estate() []Fixture {
	var out []Fixture
	for _, f := range [][]Fixture{homes(), overviewPages(), settingsPages(), settingsMatrix(), disks(), fleets(), machineLists(), sshPages(), imageLists(), portLists(), outcomes(), setups(), runs(), accountPages(), backgroundPages(), controlPages(), powerPages(), awakePages(), activityPages(), menubarPages(), menubarOwnPages()} {
		out = append(out, f...)
	}
	return out
}

func check(name string, state catalog.State, value, detail string) overview.Check {
	return overview.Check{Name: name, State: state, Value: value, Detail: detail}
}

func snapshot() pagehome.Overview {
	toolReports := reports(thisMac(), false)
	return pagehome.Overview{
		GeneratedAt: built,
		Station:     overview.Station{Version: "v2.0.0-250", Daemon: check("daemon", catalog.StateDegraded, "not installed", "station setup")},
		Machine:     overview.Machine{Name: "macbook", Registered: true, Kind: catalog.Host, Hostname: "jterrazz-macbook-pro", OS: "Darwin 27.0.0", Arch: "arm64", User: "jterrazz", Shell: "zsh"},
		System:      overview.System{Uptime: "22d 14h", Battery: "100% charged", MemoryTotal: 17179869184},
		Environment: overview.Environment{
			Network: []overview.Check{check("local ip", catalog.StateOK, "192.168.1.152", "")},
			Protections: []overview.Check{
				check("filevault", catalog.StateOK, "on", ""), check("firewall", catalog.StateOK, "on", ""),
				check("sip", catalog.StateAbsent, "off", ""), check("gatekeeper", catalog.StateOK, "on", ""),
			},
			Updates: []overview.Check{
				check("macos", catalog.StateDegraded, "2 pending", "checked 22m ago"),
				check("brew", catalog.StateDegraded, "120 outdated", "station apps update --method brew"),
			},
		},
		Workspace: pagehome.Workspace{
			Disk: []overview.Disk{{Name: "homebrew", Level: catalog.LevelCache, Bytes: 26 * gb}},
			Strays: []disk.StrayReport{
				{Name: catalog.HypervisorTool, Path: "~/." + catalog.HypervisorTool, Bytes: 73 * gb, Repair: "station disk move ~/." + catalog.HypervisorTool},
				{Name: "bun", Path: "~/.bun", Bytes: 2 * gb, Repair: "station disk move ~/.bun"},
			},
		},
		Tools: toolReports,
		Config: []settings.Report{
			{Item: catalog.ConfigItem{Name: "hushlogin", Description: "Silence the login banner"}, State: catalog.StateOK, Applies: true},
			{Item: catalog.ConfigItem{Name: "restart-after-power-cut", Description: "Restarts after a power cut, and wakes for the network"}, State: catalog.StateDegraded, Detail: "partly applied", Applies: true},
		},
		Setup:    overview.Setup{State: catalog.StateOK, Steps: []overview.Step{{Name: "name", State: catalog.StateOK, Detail: "macbook"}}},
		Identity: []overview.Check{check("git-email", catalog.StateOK, "admin@jterrazz.com", ""), check("git-signing", catalog.StateOK, "on", "")},
		Background: []background.Task{
			{Label: "com.github.facebook.watchman", Group: background.GroupKnown, Run: background.Running, PID: 3464, Scope: "user", Owner: "watchman", Plist: "~/Library/LaunchAgents/com.github.facebook.watchman.plist", State: catalog.StateOK},
			{Label: "com.jterrazz.brain.x-snapshot", Group: background.GroupUnknown, Run: background.Failing, Exit: 2, Scope: "user", Plist: "~/Library/LaunchAgents/com.jterrazz.brain.x-snapshot.plist", State: catalog.StateBroken,
				Removal: "launchctl bootout gui/501/com.jterrazz.brain.x-snapshot && rm ~/Library/LaunchAgents/com.jterrazz.brain.x-snapshot.plist"},
			{Label: "com.docker.socket", Group: background.GroupOrphan, Run: background.NotLoaded, Scope: "root", Owner: "Docker.app", OwnerGone: true, Plist: "/Library/LaunchDaemons/com.docker.socket.plist", State: catalog.StateDegraded,
				Removal: "sudo launchctl bootout system/com.docker.socket && sudo rm /Library/LaunchDaemons/com.docker.socket.plist"},
		},
		Gateways: []overview.Check{check("Tailscale Serve", catalog.StateOK, "on", "")},
		Tailnet:  ports.Status{State: catalog.StateOK, Connected: true, BackendState: "Running", Name: "macbook", DNSName: "jterrazz-macbook-pro.tail1234.ts.net", IP: "100.98.133.14", ServeOn: true},
	}
}

func pendingSnapshot() pagehome.Overview {
	s := snapshot()
	s.System = overview.System{Pending: true}
	s.ToolsPending, s.ConfigPending = true, true
	s.Workspace.DiskPending = true
	s.Station.Daemon = overview.Check{Name: "daemon", State: catalog.StatePending}
	s.Tailnet = ports.Status{State: catalog.StatePending}
	s.Setup = overview.Setup{State: catalog.StatePending}
	for _, list := range [][]overview.Check{s.Environment.Updates, s.Identity, s.Gateways} {
		for i := range list {
			list[i] = overview.Check{Name: list[i].Name, State: catalog.StatePending}
		}
	}
	s.Background = nil
	return s
}

func calmSnapshot() pagehome.Overview {
	s := snapshot()
	s.Station.Daemon = check("daemon", catalog.StateOK, "running v2.0.0-250", "")
	s.Environment.Updates = []overview.Check{check("macos", catalog.StateOK, "up to date", "checked 22m ago"), check("brew", catalog.StateOK, "up to date", "")}
	s.Workspace.Strays = nil
	s.Tools = reports([]tool{{name: "homebrew", category: catalog.CategoryPackageManagers, method: catalog.MethodScript, system: true, state: catalog.StateOK, version: "7.0.12"}}, false)
	s.Config = s.Config[:1]
	s.Background = s.Background[:1]
	s.Gateways = nil
	return s
}

func homes() []Fixture {
	out := states("home",
		pagehome.Snapshot(pendingSnapshot(), here(), false),
		pagehome.Snapshot(calmSnapshot(), here(), false),
		failure("home", "status", "status reads the Mac it runs on", "station overview --on <vm>"),
		pagehome.Snapshot(snapshot(), here(), false))
	return append(out, states("home-all",
		pagehome.Snapshot(pendingSnapshot(), here(), true),
		pagehome.Snapshot(calmSnapshot(), here(), true),
		failure("home", "status", "the settings file is not valid JSON", "repair the JSON of ~/.config/station/settings.json"),
		pagehome.Snapshot(unnamedSnapshot(), here(), true))...)
}

// value: this Mac as it reads before setup names it, its daemon from an
// older build: the long daemon value is one row's, and no other row's detail
// gives way to it (§11.2 per-row overflow), and the setup row reads not set.
func unnamedSnapshot() pagehome.Overview {
	s := snapshot()
	s.Machine.Name, s.Machine.Registered = "", false
	s.Setup = overview.Setup{State: catalog.StateDegraded, Steps: []overview.Step{{Name: "name", State: catalog.StateDegraded}}}
	s.Station.Daemon = check("daemon", catalog.StateDegraded, "runs v2.0.0-242-gd653060, station is v2.0.0-250", "station setup")
	return s
}

// value: the settings of the estate's Mac in every Set by station state,
// and one of each other group.
func settingRows(reading bool) []settings.Row {
	since := time.Date(2026, 11, 2, 10, 0, 0, 0, time.UTC)
	states := []settings.State{settings.StateApplied, settings.StateChangedSince, settings.StateNotApplied, settings.StateOffHere, settings.StateNotForThisMachine}
	out := make([]settings.Row, 0, len(catalog.ConfigItems))
	for i, it := range catalog.ConfigItems {
		r := settings.Row{Name: it.Name, Group: settings.GroupOf(it), State: states[i%len(states)]}
		switch {
		case reading:
			r.State = settings.StateReading
		case r.State == settings.StateApplied || r.State == settings.StateChangedSince:
			r.Since = since
		case r.State == settings.StateNotForThisMachine:
			r.Why = settings.PlaceReason([]catalog.MachineKind{catalog.Linux})
		}
		if r.Group == settings.AlwaysOnMac && !reading {
			r.State, r.Since = settings.StateNotApplied, time.Time{}
		}
		if r.State == settings.StateApplied || r.State == settings.StateChangedSince {
			r.Where = "~/." + it.Name
		}
		out = append(out, r)
	}
	return append(out, checkedRows(reading)...)
}

// value: the checked settings of the estate's Mac, judged from its facts:
// one needing a fix, one past the laptop's lock time, one with no answer,
// one off here, the rest as expected.
func checkedRows(reading bool) []settings.Row {
	now := time.Date(2026, 11, 4, 10, 0, 0, 0, time.UTC)
	fact := func(state catalog.State, value string) settings.Fact {
		return settings.Fact{State: state, Value: value}
	}
	facts := map[catalog.Fact]settings.Fact{
		catalog.FactFileVault: fact(catalog.StateOK, "on"), catalog.FactFirewall: fact(catalog.StateAbsent, "off"),
		catalog.FactSIP: fact(catalog.StateOK, "on"), catalog.FactAutoUpdates: fact(catalog.StateOK, "on"),
		catalog.FactScreenLock: fact(catalog.StateOK, "0"), catalog.FactDisplaySleep: fact(catalog.StateOK, "10"),
		catalog.FactRemoteLogin:   fact(catalog.StateAbsent, "off"),
		catalog.FactTimeMachine:   {State: catalog.StateUnknown, Detail: "no answer in 2 s"},
		catalog.FactSystemUpdates: fact(catalog.StateOK, "up to date"),
	}
	expect := settings.Expectation{Laptop: true, Now: now}
	out := make([]settings.Row, 0, len(catalog.CheckedSettings))
	for _, it := range catalog.CheckedSettings {
		r := settings.JudgeChecked(it, true, facts, expect, it.Name == "gatekeeper")
		if reading {
			r = settings.Row{Name: it.Name, Group: settings.Checked, State: settings.StateReading}
		}
		out = append(out, r)
	}
	return out
}

func settingsPages() []Fixture {
	now := time.Date(2026, 11, 4, 10, 0, 0, 0, time.UTC)
	return states("settings",
		pagesettings.List(settingRows(true), now, false, here()),
		pagesettings.List(nil, now, false, here()),
		failure(pagesettings.ID, "settings", "the preferences file is not valid JSON", "repair the JSON of ~/.config/station/preferences.json"),
		pagesettings.List(settingRows(false), now, false, here()))
}

// value: the matrix of the estate: This Mac and a desktop read, a running
// workspace, a stopped one shown from its last read, and a server named apart.
func settingsMatrix() []Fixture {
	now := time.Date(2026, 11, 4, 10, 0, 0, 0, time.UTC)
	mac := settingRows(false)
	space := make([]settings.Row, 0, len(mac))
	for _, r := range mac {
		if r.Group != settings.SetByStation {
			r.State, r.Checked, r.Why = settings.StateNotForThisMachine, "", "Mac only"
			if r.Group == settings.Checked {
				r.Checked = settings.CheckedNotForThisMachine
			}
		}
		space = append(space, r)
	}
	cols := []settings.Column{
		{Place: settings.Place{Name: "jterrazz-macbook-pro", Here: true, Mac: true}, Rows: mac, Read: now},
		{Place: settings.Place{Name: "jterrazz-mac-mini", Mac: true}, Rows: mac, Read: now},
		{Place: settings.Place{Name: "linux-1"}, Rows: space, Read: now},
		{Place: settings.Place{Name: "api", Away: settings.Stopped}, Rows: space, Read: now.Add(-2 * time.Hour)},
	}
	short := func(name string) string { return strings.TrimPrefix(name, "jterrazz-") }
	m := settings.MatrixOf(cols, []string{"infra"})
	full := pagesettings.Matrix(m, now, false, short, here())
	return states("settings-matrix", pending(pagesettings.Matrix(m, now, false, short, here())),
		pagesettings.Matrix(settings.MatrixOf(nil, nil), now, false, short, here()),
		failure(pagesettings.ID, "settings --all-machines", "jterrazz-macbook-pro does not answer", "station machines"),
		full)
}

func storageReport(pending bool) disk.Survey {
	entry := func(name, category string) disk.Entry {
		return disk.Entry{Name: name, Category: category, Places: []catalog.MachineKind{catalog.Host}}
	}
	at := func(path string, level catalog.Level, bytes int64) disk.PathReport {
		return disk.PathReport{Path: path, Level: level, Bytes: bytes}
	}
	rows := make([]disk.Tally, 0, 4)
	rows = append(rows,
		disk.Tally{Entry: entry(disk.StationName, disk.StationCategory), Installed: true, State: catalog.StateOK, Bytes: map[catalog.Level]int64{catalog.LevelData: 4 * gb, catalog.LevelConfig: 12000},
			Paths: []disk.PathReport{at("~/.local/share/station/vms/vms/*", catalog.LevelData, 4*gb), at("~/.config/station/preferences.json", catalog.LevelConfig, 12000)}},
		disk.Tally{Entry: entry("homebrew", string(catalog.CategoryPackageManagers)), Installed: true, State: catalog.StateOK, Bytes: map[catalog.Level]int64{catalog.LevelCache: 26 * gb}, Install: 3 * gb,
			Paths: []disk.PathReport{at("~/Library/Caches/Homebrew", catalog.LevelCache, 26*gb)}},
		disk.Tally{Entry: entry("bun", string(catalog.CategoryRuntimes)), Installed: true, State: catalog.StateOK, Bytes: map[catalog.Level]int64{catalog.LevelCache: 1700 * (1 << 20)},
			Paths: []disk.PathReport{at("~/Library/Caches/bun", catalog.LevelCache, 1700*(1<<20))}},
	)
	misplaced := []disk.StrayReport{{Name: catalog.HypervisorTool, Path: "~/." + catalog.HypervisorTool, Bytes: 73 * gb, To: "~/.local/share/station/vms", Repair: "station disk move ~/." + catalog.HypervisorTool}}
	rows = append(rows, disk.Tally{Entry: entry(catalog.HypervisorTool, string(catalog.CategoryContainersVMs)), Installed: true, State: catalog.StateDegraded, Bytes: map[catalog.Level]int64{}, Strays: misplaced})
	r := disk.Survey{Rows: rows, Strays: misplaced, Other: 9 * gb}
	if pending {
		for i := range r.Rows {
			r.Rows[i].Pending, r.Rows[i].State = true, catalog.StatePending
		}
	}
	return r
}

func disks() []Fixture {
	reading := func(pending bool) disk.Reading {
		return disk.Reading{Total: 994e9, Free: 388e9, Rows: disk.Classify(storageReport(pending)), Pending: pending}
	}
	return states("disk",
		pagedisk.Read(reading(true), here(), false),
		pagedisk.Read(disk.Reading{Total: 994e9, Free: 388e9}, here(), false),
		failure("disk", "disk", "the walk of ~ was refused: operation not permitted", "grant Full Disk Access to the terminal"),
		pagedisk.Read(reading(false), here(), false))
}

func fleet() machines.Fleet {
	return machines.Fleet{Hosts: []machines.HostView{
		{Name: "macbook", User: "jterrazz", StationVersion: "v2.0.0-250", Here: true, Reachable: true, Daemon: machines.DaemonInfo{Known: true, Running: true, Version: "v2.0.0-250"}, Node: machines.Node{Online: true, IP: "100.98.133.14"},
			VMs: []machines.VM{
				sized(machines.VM{Name: "linux-1", Image: "linux", Version: "3", Host: "macbook", Kind: catalog.Linux, Lifetime: machines.Throwaway, State: machines.Running, SizeGB: 20, IP: "192.168.64.5", CreatedAt: built}, 4, 4),
				sized(machines.VM{Name: "api", Image: "linux", Version: "3", Host: "macbook", Kind: catalog.Linux, Lifetime: machines.Permanent, State: machines.Stopped, SizeGB: 40, CreatedAt: built}, 4, 8),
			}},
		{Name: "mac-mini", User: "jterrazz", StationVersion: "v2.0.0-248", Reachable: true, Daemon: machines.DaemonInfo{Known: true, Running: true, Version: "v2.0.0-248"}, Node: machines.Node{Online: true, IP: "100.98.133.20"}},
	}}
}

// value: the fleet whose daemon socket refuses: the agent's plist is there,
// so this host reads stopped (R9.D3); the mini does not answer its ssh read.
func stoppedFleet() machines.Fleet {
	f := fleet()
	f.Hosts[0].Daemon = machines.DaemonInfo{Known: true, Installed: true}
	f.Hosts[1] = machines.HostView{Name: "mac-mini", User: "jterrazz", Node: machines.Node{IP: "100.98.133.20"}}
	return f
}

func sized(vm machines.VM, cpus int, memoryGB int64) machines.VM {
	vm.Resources.CPUs, vm.Resources.MemoryBytes, vm.Resources.DiskBytes = cpus, memoryGB*gb, 40*gb
	return vm
}

func machineLists() []Fixture {
	return states("machines",
		pending(pagemachines.List(fleet(), nil, here())),
		pagemachines.List(machines.Fleet{}, nil, here()),
		pagemachines.List(stoppedFleet(), nil, here()),
		pagemachines.List(fleet(), []string{"mac-mini: station v2.0.0-248, This Mac v2.0.0-250"}, here()))
}

// value: infra, a server of mac-mini, as This Mac reads it for its SSH
// section: running, then stopped.
func server() pagemachines.VMOverview {
	f := fleet()
	vm := sized(machines.VM{Name: "infra", Image: "linux", Version: "3", Host: "mac-mini", Kind: catalog.Linux, Lifetime: machines.AlwaysOn, State: machines.Running, SizeGB: 80, IP: "192.168.64.7", Want: "running", Check: "https://infra:6443/readyz", Device: "infra", CreatedAt: built}, 8, 24)
	f.Hosts[1].VMs = []machines.VM{vm}
	f.Hosts[1].User = "jterrazz.agent"
	return pagemachines.VMOverview{Fleet: f, Mac: f.Hosts[1], VM: vm}
}

func sshPages() []Fixture {
	running, stopped := server(), server()
	stopped.VM.State, stopped.VM.IP = machines.Stopped, ""
	target := machines.SSHTarget{User: "admin", Host: "infra", Command: "ssh admin@infra"}
	return states("ssh",
		pending(pagemachines.SSHPage(running, target, here())),
		pagemachines.SSHPage(stopped, target, here()),
		failure("ssh", "machines ssh infra", "infra is on mac-mini, which does not answer", "station machines"),
		pagemachines.SSHPage(running, target, here()))
}

func imageReports() []images.Report {
	return []images.Report{
		{Record: images.Record{Name: "linux", Version: "2026.11.01", Base: "ghcr.io/cirruslabs/ubuntu:24.04", Station: "v2.0.0-250", BuiltAt: built, SizeGB: 4}, Newest: true, VMs: []string{"linux-1", "api"}, Mac: "jterrazz-macbook-pro"},
		{Record: images.Record{Name: "linux", Version: "2026.10.20", Base: "ghcr.io/cirruslabs/ubuntu:24.04", Station: "v2.0.0-240", BuiltAt: built.AddDate(0, 0, -20), SizeGB: 4}, Mac: "jterrazz-macbook-pro"},
		{Record: images.Record{Name: "linux", Version: "2026.11.01", Base: "ghcr.io/cirruslabs/ubuntu:24.04", Station: "v2.0.0-250", BuiltAt: built, SizeGB: 4}, Newest: true, VMs: []string{"infra"}, Mac: "jterrazz-mac-mini"},
	}
}

func imageLists() []Fixture {
	return states("images",
		pending(pageimages.List(imageReports(), thisMacImages())),
		pageimages.List(nil, thisMacImages()),
		failure("images", "images", catalog.HypervisorTool+" is not installed", "station apps install "+catalog.HypervisorTool),
		pageimages.List(imageReports(), thisMacImages()))
}

func thisMacImages() page.Context {
	return page.Context{Kind: "host", Machine: "jterrazz-macbook-pro", Registered: true}
}

func tailnetStatus() ports.Status {
	return ports.Status{
		State: catalog.StateOK, Connected: true, BackendState: "Running", Name: "macbook", DNSName: "jterrazz-macbook-pro.tail1234.ts.net",
		IP: "100.98.133.14", Variant: "app-store", CLI: "/Applications/Tailscale.app/Contents/MacOS/Tailscale", ServeOn: true,
		Peers: []ports.Node{
			{ID: "1", Name: "mac-mini", IP: "100.98.133.20", Online: true, Tags: []string{"tag:station"}},
			{ID: "2", Name: "iphone", IP: "100.98.133.30"},
		},
		Serve: []ports.Served{{Share: ports.Share{Port: 40001, Path: "/", Scheme: ports.SchemeHTTPS, Target: "http://127.0.0.1:40001"}, Station: true}},
	}
}

func serviceFleet() ports.Fleet {
	seen := built
	return ports.Fleet{Schema: "station.services/v1", Hosts: []ports.HostServices{
		{Name: "macbook", Here: true, Reachable: true, Daemon: ports.EngineInfo{Running: true}, PublicHost: "jterrazz-macbook-pro.tail1234.ts.net", Serve: "on", HTTPS: "port",
			Range: &ports.Range{Low: 40000, High: 40999, Leased: 2, Free: 998, MaxPerVM: 20},
			Services: []ports.ServiceView{
				{VM: "linux-1", GuestPort: 5173, HostPort: 40001, State: ports.PhasePublished, Process: "node", HTTP: true, Profile: "vite", URLs: ports.URLSet{Public: "https://jterrazz-macbook-pro.tail1234.ts.net:40001"}, LastSeenAt: &seen},
				{VM: "linux-1", GuestPort: 5432, HostPort: 40002, State: ports.PhaseWaiting, Process: "postgres"},
			}},
		{Name: "mac-mini", Reachable: false},
	}}
}

func stoppedServices() ports.Fleet {
	f := serviceFleet()
	f.Hosts[0].Daemon = ports.EngineInfo{Installed: true}
	for i := range f.Hosts[0].Services {
		f.Hosts[0].Services[i].State = ports.PhaseUnknown
	}
	return f
}

func thisMacPorts() page.Context { return page.Context{Kind: "host", Machine: "macbook"} }

func portLists() []Fixture {
	return states("ports",
		pending(pageports.List(ports.Listing{Fleet: serviceFleet(), Tailnet: tailnetStatus()}, thisMacPorts())),
		pageports.List(ports.Listing{Tailnet: ports.Status{State: catalog.StateAbsent}}, thisMacPorts()),
		pageports.List(ports.Listing{Fleet: stoppedServices(), Tailnet: ports.Status{State: catalog.StateOK, BackendState: "NeedsLogin"}}, thisMacPorts()),
		pageports.List(ports.Listing{Fleet: estatePorts(), Tailnet: tailnetStatus()}, thisMacPorts()))
}

// value: the reference estate's ports in their two classes: the mini's AirPlay
// Receiver red, infra's API on its own tailnet device, a database on This
// Mac's loopback folded.
func estatePorts() ports.Fleet {
	f := serviceFleet()
	mini, infra := coremachine.Ref{Name: "mac-mini"}, coremachine.Ref{Name: "infra", On: "mac-mini"}
	f.Hosts[0].Ports = []ports.Port{
		{Machine: coremachine.Ref{Name: "macbook"}, Port: 5432, Process: "postgres", Binds: []string{"127.0.0.1"}, Reach: ports.ThisMachine},
		{Machine: coremachine.Ref{Name: "linux-1", On: "macbook"}, Port: 5173, Process: "node", Binds: []string{"0.0.0.0"}, Reach: ports.Tailnet},
	}
	f.Hosts[1] = ports.HostServices{Name: "mac-mini", Reachable: true, Daemon: ports.EngineInfo{Running: true}, Services: []ports.ServiceView{},
		Range: &ports.Range{Low: 40000, High: 40999, Free: 1000, MaxPerVM: 20},
		Ports: []ports.Port{
			{Machine: mini, Port: 5000, Process: "ControlCenter", Binds: []string{"*"}, Reach: ports.Outside},
			{Machine: mini, Port: 22, Process: "sshd", Binds: []string{"*"}, Reach: ports.Outside, Declared: &ports.Declaration{Machine: mini.Name, Port: 22, Via: catalog.RouteLAN, By: "sshd"}, Verified: &ports.Verified{Seen: true}},
			{Machine: infra, Port: 6443, Process: "kube-apiserver", Binds: []string{"0.0.0.0"}, Reach: ports.Tailnet, Device: true},
		}}
	return f
}

func fleets() []Fixture {
	f, list, svc, tn := fleet(), imageReports(), serviceFleet(), tailnetStatus()
	full := pagefleet.Reading{Machines: &f, Images: &list, Services: &svc, Tailnet: &tn}
	none := machines.Fleet{Hosts: []machines.HostView{{Name: "macbook", Here: true, Reachable: true}}}
	var noImages []images.Report
	empty := pagefleet.Reading{Machines: &none, Images: &noImages, Tailnet: &ports.Status{State: catalog.StateAbsent}}
	stopped, stoppedSvc := stoppedFleet(), stoppedServices()
	failed := pagefleet.Reading{Machines: &stopped, Images: &list, Services: &stoppedSvc, Tailnet: &tn}
	return states("fleet",
		pagefleet.Tab(pagefleet.Reading{}, here()),
		pagefleet.Tab(empty, here()),
		pagefleet.Tab(failed, here()),
		pagefleet.Tab(full, here()))
}

func outcomes() []Fixture {
	appOutcomes := []apps.Outcome{
		{Name: "jq", Verb: "install", Result: catalog.StateOK, Method: catalog.MethodBrew, Version: "1.8.1"},
		{Name: "mise", Verb: "install", Result: catalog.StateOK, Method: catalog.MethodBrew, Version: "2026.10.1", Steps: []apps.Step{{Item: "activate", Result: catalog.StateDegraded, Detail: "restart the shell"}}},
		{Name: "typewhisper", Verb: "install", Result: catalog.StateBroken, Method: catalog.MethodCask, Detail: "brew bundle exited 1"},
	}
	changes := []settings.Change{
		{Item: "restart-after-power-cut", Verb: "apply", Result: catalog.StateOK, Before: []settings.KeyValue{{Key: "autorestart", Value: "0"}}, After: []settings.KeyValue{{Key: "autorestart", Value: "1"}}},
		{Item: catalog.HypervisorLauncher, Verb: "apply", Result: catalog.StateOK},
	}
	cleaned := []disk.Outcome{
		{Name: "homebrew", Result: catalog.StateOK, Freed: 26 * gb},
		{Name: "bun", Result: catalog.StateAbsent, Detail: "nothing to free"},
	}
	out := states("outcome-apps-install",
		pending(pageapps.Outcomes("install", appOutcomes, pageapps.Run{Took: 9 * time.Second}, here())),
		pageapps.Outcomes("install", nil, pageapps.Run{}, here()),
		failure("outcome", "apps install", "no app named \"jqq\"", "station apps for every app here"),
		pageapps.Outcomes("install", appOutcomes, pageapps.Run{Took: 9 * time.Second}, here()))
	out = append(out, states("outcome-settings-apply",
		pending(pagesettings.Changes("apply", changes, here())),
		pagesettings.Changes("apply", nil, here()),
		failure("outcome", "settings apply", "restart-after-power-cut needs admin rights", "sudo station settings apply restart-after-power-cut"),
		pagesettings.Changes("apply", changes, here()))...)
	return append(out, states("outcome-disk-free",
		pending(pagedisk.Freed(cleaned, here())),
		pagedisk.Freed(nil, here()),
		failure("outcome", "disk free", "no disk row for \"brw\"", "station disk"),
		pagedisk.Freed(cleaned, here()))...)
}

func setupReport() setup.Report {
	return setup.Report{Steps: []setup.Step{
		{Name: "name", State: catalog.StateOK, Detail: "macbook"},
		{Name: "zshrc", State: catalog.StateOK, Detail: "the station block", Changed: true},
		{Name: "layout", State: catalog.StateDegraded, Detail: "~/." + catalog.HypervisorTool + " not moved", Log: "73 GB: move it with station setup --move"},
		{Name: "apps", State: catalog.StateOK},
	}}
}

func setups() []Fixture {
	return states("setup",
		pending(pagesetup.Outcome(setupReport(), here())),
		pagesetup.Outcome(setup.Report{}, here()),
		failure("outcome", "setup", "a VM is named by its Mac", "station setup on its Mac"),
		pagesetup.Outcome(setupReport(), here()))
}

func runs() []Fixture {
	run := settings.ContainersOutcome{Shortcut: "dreset", Removals: []settings.ContainersRemoval{{What: "containers", Count: 3}, {What: "images", Count: 12}}}
	return states("run",
		pending(pagesettings.Removed(run, "run dreset", here())),
		pagesettings.Removed(settings.ContainersOutcome{Shortcut: "dreset"}, "run dreset", here()),
		failure("outcome", "run dreset", "docker ps failed: Docker is not running", "start Docker, then run it again"),
		pagesettings.Removed(run, "run dreset", here()))
}
