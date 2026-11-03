package pages

import (
	"errors"
	"time"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/domain/config"
	"github.com/jterrazz/studio-station/src/domain/images"
	"github.com/jterrazz/studio-station/src/domain/machines"
	"github.com/jterrazz/studio-station/src/domain/secrets"
	"github.com/jterrazz/studio-station/src/domain/services"
	"github.com/jterrazz/studio-station/src/domain/setup"
	"github.com/jterrazz/studio-station/src/domain/shortcuts"
	"github.com/jterrazz/studio-station/src/domain/status"
	"github.com/jterrazz/studio-station/src/domain/storage"
	"github.com/jterrazz/studio-station/src/domain/tailnet"
	"github.com/jterrazz/studio-station/src/domain/tools"
	"github.com/jterrazz/studio-station/src/page"
	allpages "github.com/jterrazz/studio-station/src/pages"
	pageconfig "github.com/jterrazz/studio-station/src/pages/config"
	pagefleet "github.com/jterrazz/studio-station/src/pages/fleet"
	pagehome "github.com/jterrazz/studio-station/src/pages/home"
	pageimages "github.com/jterrazz/studio-station/src/pages/images"
	pagemachines "github.com/jterrazz/studio-station/src/pages/machines"
	pagesecrets "github.com/jterrazz/studio-station/src/pages/secrets"
	pageservices "github.com/jterrazz/studio-station/src/pages/services"
	pagesetup "github.com/jterrazz/studio-station/src/pages/setup"
	pageshortcuts "github.com/jterrazz/studio-station/src/pages/shortcuts"
	pagestorage "github.com/jterrazz/studio-station/src/pages/storage"
	pagetailnet "github.com/jterrazz/studio-station/src/pages/tailnet"
	pagetools "github.com/jterrazz/studio-station/src/pages/tools"
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
	for _, f := range [][]Fixture{homes(), configs(), storages(), fleets(), machineLists(), imageLists(), secretLists(), tailnets(), serviceLists(), outcomes(), setups(), runs(), accountPages(), controlPages()} {
		out = append(out, f...)
	}
	return out
}

func check(name string, state catalog.State, value, detail string) status.Check {
	return status.Check{Name: name, State: state, Value: value, Detail: detail}
}

func snapshot() status.Snapshot {
	toolReports := reports(thisMac(), false)
	return status.Snapshot{
		GeneratedAt: built,
		Station:     status.Station{Version: "v2.0.0-250", Daemon: check("daemon", catalog.StateDegraded, "not installed", "station setup")},
		Machine:     status.Machine{Name: "macbook", Registered: true, Kind: catalog.Host, Hostname: "jterrazz-macbook-pro", OS: "Darwin 27.0.0", Arch: "arm64", User: "jterrazz", Shell: "zsh"},
		System:      status.System{Uptime: "22d 14h", Battery: "100% charged", MemoryTotal: 17179869184},
		Environment: status.Environment{
			Network: []status.Check{check("local ip", catalog.StateOK, "192.168.1.152", ""), check("vpn", catalog.StateOK, "Tailscale", "")},
			Security: []status.Check{
				check("filevault", catalog.StateOK, "on", ""), check("firewall", catalog.StateOK, "on", ""),
				check("sip", catalog.StateAbsent, "off", ""), check("gatekeeper", catalog.StateOK, "on", ""),
			},
			Updates: []status.Check{
				check("macos", catalog.StateDegraded, "2 pending", "checked 22m ago"),
				check("brew", catalog.StateDegraded, "120 outdated", "station tools upgrade --via brew"),
			},
		},
		Workspace: status.Workspace{
			Repositories: []status.Repository{{Path: "jterrazz/station", Branch: "main"}, {Path: "jterrazz/web", Branch: "main", Dirty: 2}},
			Dependencies: []status.Dependency{{Path: "jterrazz/web", Manager: "pnpm", Outdated: 14}},
			Docker:       status.Docker{State: catalog.StateDegraded},
			Disk:         []status.Disk{{Name: "homebrew", Level: catalog.LevelCache, Bytes: 26 * gb}},
			Strays: []storage.StrayReport{
				{Name: "tart", Path: "~/.tart", Bytes: 73 * gb, Repair: "station setup"},
				{Name: "bun", Path: "~/.bun", Bytes: 2 * gb, Repair: "station setup"},
			},
		},
		Tools: toolReports,
		Config: []config.Report{
			{Item: catalog.ConfigItem{Name: "hushlogin", Description: "Silence the login banner"}, State: catalog.StateOK, Applies: true},
			{Item: catalog.ConfigItem{Name: "power", Description: "Always-on power policy"}, State: catalog.StateDegraded, Detail: "policy partly applied", Applies: true},
		},
		Setup:    status.Setup{State: catalog.StateOK, Steps: []setup.Step{{Name: "name", State: catalog.StateOK, Detail: "macbook"}}},
		Identity: []status.Check{check("git-email", catalog.StateOK, "admin@jterrazz.com", ""), check("git-signing", catalog.StateOK, "on", "")},
		Background: []status.Job{
			{Label: "com.github.facebook.watchman", Group: status.GroupKnown, Run: status.JobRunning, PID: 3464, Scope: "user", Owner: "watchman", Plist: "~/Library/LaunchAgents/com.github.facebook.watchman.plist", State: catalog.StateOK},
			{Label: "com.jterrazz.brain.x-snapshot", Group: status.GroupUnknown, Run: status.JobFailing, Exit: 2, Scope: "user", Plist: "~/Library/LaunchAgents/com.jterrazz.brain.x-snapshot.plist", State: catalog.StateBroken,
				Removal: "launchctl bootout gui/501/com.jterrazz.brain.x-snapshot && rm ~/Library/LaunchAgents/com.jterrazz.brain.x-snapshot.plist"},
			{Label: "com.docker.socket", Group: status.GroupOrphan, Run: status.JobNotLoaded, Scope: "root", Owner: "Docker.app", OwnerGone: true, Plist: "/Library/LaunchDaemons/com.docker.socket.plist", State: catalog.StateDegraded,
				Removal: "sudo launchctl bootout system/com.docker.socket && sudo rm /Library/LaunchDaemons/com.docker.socket.plist"},
		},
		Gateways: []status.Check{check("Hermes runtime", catalog.StateBroken, "not running", "gateway process not found")},
		Tailnet:  tailnet.Status{State: catalog.StateOK, Connected: true, BackendState: "Running", Name: "macbook", DNSName: "jterrazz-macbook-pro.tail1234.ts.net", IP: "100.98.133.14", ServeOn: true},
	}
}

func pendingSnapshot() status.Snapshot {
	s := snapshot()
	s.System = status.System{Pending: true}
	s.ToolsPending, s.ConfigPending = true, true
	s.Workspace.ReposPending, s.Workspace.DepsPending, s.Workspace.DiskPending = true, true, true
	s.Workspace.Docker.State = catalog.StatePending
	s.Station.Daemon = status.Check{Name: "daemon", State: catalog.StatePending}
	s.Tailnet = tailnet.Status{State: catalog.StatePending}
	s.Setup = status.Setup{State: catalog.StatePending}
	for _, list := range [][]status.Check{s.Environment.Updates, s.Identity, s.Gateways} {
		for i := range list {
			list[i] = status.Check{Name: list[i].Name, State: catalog.StatePending}
		}
	}
	s.Background = nil
	return s
}

func calmSnapshot() status.Snapshot {
	s := snapshot()
	s.Station.Daemon = check("daemon", catalog.StateOK, "running v2.0.0-250", "")
	s.Environment.Updates = []status.Check{check("macos", catalog.StateOK, "up to date", "checked 22m ago"), check("brew", catalog.StateOK, "up to date", "")}
	s.Workspace.Dependencies, s.Workspace.Strays = nil, nil
	s.Workspace.Docker.State = catalog.StateOK
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
		failure("home", "status", "status reads the host it runs on", "station status --on <vm>"),
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
func unnamedSnapshot() status.Snapshot {
	s := snapshot()
	s.Machine.Name, s.Machine.Registered = "", false
	s.Setup = status.Setup{State: catalog.StateDegraded, Steps: []setup.Step{{Name: "name", State: catalog.StateDegraded}}}
	s.Station.Daemon = check("daemon", catalog.StateDegraded, "runs v2.0.0-242-gd653060, station is v2.0.0-250", "station setup")
	return s
}

func configReports(state catalog.State) []config.Report {
	items := catalog.ConfigItems
	if len(items) > 6 {
		items = items[:6]
	}
	out := make([]config.Report, 0, len(items))
	for i, it := range items {
		r := config.Report{Item: it, State: state, Applies: true}
		if state != catalog.StatePending {
			switch i % 3 {
			case 1:
				r.State, r.Detail = catalog.StateAbsent, ""
			case 2:
				r.State, r.Detail = catalog.StateDegraded, "differs from station's"
			}
		}
		out = append(out, r)
	}
	return out
}

func configs() []Fixture {
	return states("config",
		pageconfig.Items(configReports(catalog.StatePending), here()),
		pageconfig.Items(nil, here()),
		failure("config", "config", "the settings file is not valid JSON", "repair the JSON of ~/.config/station/settings.json"),
		pageconfig.Items(configReports(catalog.StateOK), here()))
}

func storageReport(pending bool) storage.Report {
	entry := func(name, category string) storage.Entry {
		return storage.Entry{Name: name, Category: category, Places: []catalog.MachineKind{catalog.Host}}
	}
	rows := []storage.Row{
		{Entry: entry(storage.StationName, storage.StationCategory), Installed: true, State: catalog.StateOK, Bytes: map[catalog.Level]int64{catalog.LevelData: 4 * gb, catalog.LevelConfig: 12000}},
		{Entry: entry("homebrew", string(catalog.CategoryPackageManagers)), Installed: true, State: catalog.StateOK, Bytes: map[catalog.Level]int64{catalog.LevelCache: 26 * gb}, Install: 3 * gb},
		{Entry: entry("bun", string(catalog.CategoryRuntimes)), Installed: true, State: catalog.StateOK, Bytes: map[catalog.Level]int64{catalog.LevelCache: 1700 * (1 << 20)}},
	}
	r := storage.Report{Rows: rows, Strays: []storage.StrayReport{{Name: "tart", Path: "~/.tart", Bytes: 73 * gb, Repair: "station setup"}}, Other: 9 * gb}
	if pending {
		for i := range r.Rows {
			r.Rows[i].Pending, r.Rows[i].State = true, catalog.StatePending
		}
	}
	return r
}

func storages() []Fixture {
	return states("storage",
		pagestorage.List(storageReport(true), here()),
		pagestorage.List(storage.Report{}, here()),
		failure("storage", "storage", "the walk of ~ was refused: operation not permitted", "grant Full Disk Access to the terminal"),
		pagestorage.List(storageReport(false), here()))
}

func fleet() machines.Fleet {
	return machines.Fleet{Hosts: []machines.HostView{
		{Name: "macbook", User: "jterrazz", StationVersion: "v2.0.0-250", Here: true, Reachable: true, Daemon: machines.DaemonInfo{Known: true, Running: true, Version: "v2.0.0-250"}, Node: machines.Node{Online: true, IP: "100.98.133.14"},
			VMs: []machines.VM{
				sized(machines.VM{Name: "linux-1", Image: "linux", Version: "3", Host: "macbook", Kind: catalog.Linux, Lifetime: machines.Temporary, State: machines.Running, SizeGB: 20, IP: "192.168.64.5", CreatedAt: built}, 4, 4),
				sized(machines.VM{Name: "api", Image: "linux", Version: "3", Host: "macbook", Kind: catalog.Linux, Lifetime: machines.Lasting, State: machines.Stopped, SizeGB: 40, CreatedAt: built}, 4, 8),
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
		pagemachines.List(fleet(), []string{"mac-mini: station v2.0.0-248, here v2.0.0-250"}, here()))
}

func imageReports() []images.Report {
	return []images.Report{
		{Record: images.Record{Name: "linux", Version: "3", Base: "ghcr.io/cirruslabs/ubuntu:24.04", Station: "v2.0.0-250", BuiltAt: built, SizeGB: 6}, Newest: true, VMs: []string{"linux-1", "api"}},
		{Record: images.Record{Name: "linux", Version: "2", Base: "ghcr.io/cirruslabs/ubuntu:24.04", Station: "v2.0.0-240", BuiltAt: built.AddDate(0, 0, -20), SizeGB: 6}},
		{Record: images.Record{Name: "macos", Version: "1", Base: "ghcr.io/cirruslabs/macos-tahoe-base:latest", Station: "v2.0.0-250", BuiltAt: built, SizeGB: 48}, Newest: true},
	}
}

func imageLists() []Fixture {
	return states("images",
		pending(pageimages.List(imageReports(), here())),
		pageimages.List(nil, here()),
		failure("images", "images", "tart is not installed", "station tools install tart"),
		pageimages.List(imageReports(), here()))
}

func secretReport() secrets.Report {
	return secrets.Report{LendList: []string{"GITHUB_TOKEN", "NPM_TOKEN"}, Lent: map[string][]string{"linux-1": {"GITHUB_TOKEN"}}, Kept: []string{"GITHUB_TOKEN", "NPM_TOKEN", "OPENAI_API_KEY"}}
}

func secretLists() []Fixture {
	return states("secrets",
		pending(pagesecrets.List(secretReport(), here())),
		pagesecrets.List(secrets.Report{}, here()),
		failure("secrets", "secrets", "the keychain is locked", "unlock the login keychain, then station secrets"),
		pagesecrets.List(secretReport(), here()))
}

func tailnetStatus() tailnet.Status {
	return tailnet.Status{
		State: catalog.StateOK, Connected: true, BackendState: "Running", Name: "macbook", DNSName: "jterrazz-macbook-pro.tail1234.ts.net",
		IP: "100.98.133.14", Variant: "app-store", CLI: "/Applications/Tailscale.app/Contents/MacOS/Tailscale", ServeOn: true,
		Peers: []tailnet.Peer{
			{ID: "1", Name: "mac-mini", IP: "100.98.133.20", Online: true, Tags: []string{"tag:station"}},
			{ID: "2", Name: "iphone", IP: "100.98.133.30"},
		},
		Serve: []tailnet.ServeEntry{{Port: 40001, Path: "/", Target: "http://127.0.0.1:40001", Station: true}},
	}
}

func tailnets() []Fixture {
	return states("tailnet",
		pagetailnet.Status(tailnet.Status{State: catalog.StatePending}, here()),
		pagetailnet.Status(tailnet.Status{State: catalog.StateOK, Connected: true, BackendState: "Running", Name: "macbook", DNSName: "jterrazz-macbook-pro.tail1234.ts.net", IP: "100.98.133.14", Variant: "app-store", CLI: "/Applications/Tailscale.app/Contents/MacOS/Tailscale", ServeOn: true}, here()),
		pagetailnet.Status(tailnet.Status{State: catalog.StateAbsent}, here()),
		pagetailnet.Status(tailnetStatus(), here()))
}

func serviceFleet() services.Fleet {
	seen := built
	return services.Fleet{Schema: "station.services/v1", Hosts: []services.HostView{
		{Name: "macbook", Here: true, Reachable: true, Daemon: services.DaemonInfo{Running: true}, PublicHost: "jterrazz-macbook-pro.tail1234.ts.net", Serve: "on", HTTPS: "port",
			Range: &services.Range{Low: 40000, High: 40999, Leased: 2, Free: 998, MaxPerVM: 20},
			Services: []services.View{
				{VM: "linux-1", GuestPort: 5173, HostPort: 40001, State: services.Published, Process: "node", HTTP: true, Profile: "vite", URLs: services.URLSet{Public: "https://jterrazz-macbook-pro.tail1234.ts.net:40001"}, LastSeenAt: &seen},
				{VM: "linux-1", GuestPort: 5432, HostPort: 40002, State: services.Waiting, Process: "postgres"},
			}},
		{Name: "mac-mini", Reachable: false},
	}}
}

func stoppedServices() services.Fleet {
	f := serviceFleet()
	f.Hosts[0].Daemon = services.DaemonInfo{Installed: true}
	for i := range f.Hosts[0].Services {
		f.Hosts[0].Services[i].State = services.Unknown
	}
	return f
}

func serviceLists() []Fixture {
	return states("services",
		pending(pageservices.Fleet(serviceFleet(), here())),
		pageservices.Fleet(services.Fleet{}, here()),
		pageservices.Fleet(stoppedServices(), here()),
		pageservices.Fleet(serviceFleet(), here()))
}

func fleets() []Fixture {
	f, list, sec, svc, tn := fleet(), imageReports(), secretReport(), serviceFleet(), tailnetStatus()
	full := pagefleet.Reading{Machines: &f, Images: &list, Secrets: &sec, Services: &svc, Tailnet: &tn}
	none := machines.Fleet{Hosts: []machines.HostView{{Name: "macbook", Here: true, Reachable: true}}}
	var noImages []images.Report
	empty := pagefleet.Reading{Machines: &none, Images: &noImages, Secrets: &secrets.Report{}, Tailnet: &tailnet.Status{State: catalog.StateAbsent}}
	stopped, stoppedSvc := stoppedFleet(), stoppedServices()
	failed := pagefleet.Reading{Machines: &stopped, Images: &list, Secrets: &sec, Services: &stoppedSvc, Tailnet: &tn}
	return states("fleet",
		pagefleet.Tab(pagefleet.Reading{}, here()),
		pagefleet.Tab(empty, here()),
		pagefleet.Tab(failed, here()),
		pagefleet.Tab(full, here()))
}

func outcomes() []Fixture {
	toolOutcomes := []tools.Outcome{
		{Name: "jq", Verb: "install", Result: catalog.StateOK, Detail: "1.8.1"},
		{Name: "mise", Verb: "install", Result: catalog.StateOK, Steps: []tools.Step{{Item: "activate", Result: catalog.StateDegraded, Detail: "restart the shell"}}},
		{Name: "typewhisper", Verb: "install", Result: catalog.StateBroken, Detail: "brew bundle exited 1", Err: errors.New("brew bundle exited 1")},
	}
	changes := []config.Change{
		{Item: "power", Verb: "install", Result: catalog.StateOK, Before: []config.Fact{{Key: "sleep", Value: "10"}}, After: []config.Fact{{Key: "sleep", Value: "0"}}},
		{Item: "tart-launcher", Verb: "install", Result: catalog.StateOK},
	}
	cleaned := []storage.Outcome{
		{Name: "homebrew", Result: catalog.StateOK, Freed: 26 * gb},
		{Name: "bun", Result: catalog.StateAbsent, Detail: "nothing to clean"},
	}
	out := states("outcome-tools-install",
		pending(pagetools.Outcomes("install", toolOutcomes, here())),
		pagetools.Outcomes("install", nil, here()),
		failure("outcome", "tools install", "no tool named jqq", "station tools"),
		pagetools.Outcomes("install", toolOutcomes, here()))
	out = append(out, states("outcome-config",
		pending(pageconfig.Changes("install", changes, here())),
		pageconfig.Changes("install", nil, here()),
		failure("outcome", "config install", "power needs root", "run it again; it asks for your password"),
		pageconfig.Changes("install", changes, here()))...)
	return append(out, states("outcome-storage",
		pending(pagestorage.Outcomes("clean", cleaned, here())),
		pagestorage.Outcomes("clean", nil, here()),
		failure("outcome", "storage clean", "no tool named brw", "station storage"),
		pagestorage.Outcomes("clean", cleaned, here()))...)
}

func setupReport() setup.Report {
	return setup.Report{Steps: []setup.Step{
		{Name: "name", State: catalog.StateOK, Detail: "macbook"},
		{Name: "zshrc", State: catalog.StateOK, Detail: "the station block", Changed: true},
		{Name: "layout", State: catalog.StateDegraded, Detail: "~/.tart left in place", Log: "73 GB: move it with station setup --move"},
		{Name: "tools", State: catalog.StateOK},
	}}
}

func setups() []Fixture {
	return states("setup",
		pending(pagesetup.Outcome(setupReport(), here())),
		pagesetup.Outcome(setup.Report{}, here()),
		failure("outcome", "setup", "a VM is named by its host", "station setup on the host"),
		pagesetup.Outcome(setupReport(), here()))
}

func runs() []Fixture {
	run := shortcuts.Outcome{Shortcut: "clean", Removals: []shortcuts.Removal{{What: "containers", Count: 3}, {What: "images", Count: 12}, {What: "volumes"}}}
	return states("run",
		pending(pageshortcuts.Docker(run, here())),
		pageshortcuts.Docker(shortcuts.Outcome{Shortcut: "clean"}, here()),
		failure("outcome", "run docker clean", "docker is not running", "open OrbStack, then station run docker clean"),
		pageshortcuts.Docker(run, here()))
}
