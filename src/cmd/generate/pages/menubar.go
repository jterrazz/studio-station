package pages

import (
	"encoding/json"
	"slices"
	"strings"
	"time"

	"github.com/jterrazz/studio-station/src/activity"
	"github.com/jterrazz/studio-station/src/awake"
	"github.com/jterrazz/studio-station/src/catalog"
	coremachine "github.com/jterrazz/studio-station/src/core/machine"
	"github.com/jterrazz/studio-station/src/domain/accounts"
	"github.com/jterrazz/studio-station/src/domain/machines"
	"github.com/jterrazz/studio-station/src/domain/power"
	"github.com/jterrazz/studio-station/src/domain/services"
	"github.com/jterrazz/studio-station/src/engine"
	"github.com/jterrazz/studio-station/src/overview"
	"github.com/jterrazz/studio-station/src/page"
	pagemenubar "github.com/jterrazz/studio-station/src/pages/menubar"
	"github.com/jterrazz/studio-station/src/ports"
	"github.com/jterrazz/studio-station/src/ui/token"
)

func samples() []activity.Reading {
	return readings(16*gb, 0, 0)
}

// value: 16 readings 2 s apart of a machine with this memory and disk,
// its CPU climbing and falling as vision 07's sparkline draws it.
func readings(memory, disk, free int64) []activity.Reading {
	cpu := []float64{12, 18, 25, 34, 30, 22, 41, 55, 48, 36, 28, 31, 44, 38, 29, 34}
	top := activity.Busiest{
		CPU:    []activity.Process{{PID: 812, Name: "station", Value: cpu[len(cpu)-1]}},
		Memory: []activity.Process{{PID: 812, Name: catalog.HypervisorTool, Value: 12.4e9}},
	}
	used := memory * 31 / 40
	at := func(i int) time.Time { return powerNow.Add(time.Duration(i-len(cpu)) * 2 * time.Second) }
	out := make([]activity.Reading, 0, len(cpu)+1)
	out = append(out, activity.Reading{Sample: activity.Sample{At: at(0), CPUTotal: 1000, MemUsed: used, MemTotal: memory, DiskTotal: disk, DiskFree: free}, Top: top})
	for i, c := range cpu {
		prev := out[len(out)-1].Sample
		out = append(out, activity.Reading{Sample: activity.Sample{
			At: at(i + 1), CPUBusy: prev.CPUBusy + uint64(c*10), CPUTotal: prev.CPUTotal + 1000, MemUsed: used + int64(i%4)*gb/4, MemTotal: memory,
			NetIn: prev.NetIn + 2_000_000 + uint64(c*40_000), NetOut: prev.NetOut + uint64(200_000), DiskTotal: disk, DiskFree: free,
		}, Top: top})
	}
	return out
}

var Levels pagemenubar.Levels // value: the confirmation rule the machine pages draw, set by the generator that owns the calls (confirm.For).

func menubarHere() page.Context { return page.Context{Machine: thisMacName, Kind: "host"} }

const (
	thisMacName = "jterrazz-macbook-pro"
	miniName    = "jterrazz-mac-mini"
)

// value: the two-macs world of USE-CASES §0.2 as the menu bar reads it:
// This Mac with linux-1 (running, two ports shared) and api (stopped), the
// mini with infra, a healthy server.
func twoMacs() machines.Fleet {
	linux1 := sized(machines.VM{Name: "linux-1", Image: "linux", Version: "3", Host: thisMacName, Kind: catalog.Linux, Lifetime: machines.Throwaway, State: machines.Running, SizeGB: 20, IP: "192.168.64.5", Guest: "connected", CreatedAt: built}, 4, 4)
	api := sized(machines.VM{Name: "api", Image: "linux", Version: "3", Host: thisMacName, Kind: catalog.Linux, Lifetime: machines.Permanent, State: machines.Stopped, SizeGB: 40, CreatedAt: built}, 4, 8)
	infra := sized(machines.VM{Name: "infra", Image: "linux", Version: "3", Host: miniName, Kind: catalog.Linux, Lifetime: machines.AlwaysOn, State: machines.Running, SizeGB: 80, IP: "192.168.64.7", Guest: "connected", Want: "running",
		Check: "https://infra:6443/readyz", Device: "infra", CreatedAt: built, CheckStatus: machines.CheckStatus{State: machines.CheckPassing, At: powerNow.Add(-40 * time.Second)}}, 8, 24)
	infra.Resources.DiskBytes = 80 * gb
	running := machines.DaemonInfo{Known: true, Running: true, Version: "v3.0.0"}
	return machines.Fleet{Hosts: []machines.HostView{
		{Name: thisMacName, User: "jterrazz", StationVersion: "v3.0.0", Here: true, Reachable: true, Daemon: running, Node: machines.Node{Online: true, IP: "100.98.133.14"}, VMs: []machines.VM{linux1, api}},
		{Name: miniName, User: "jterrazz", StationVersion: "v3.0.0", Reachable: true, Daemon: running, Node: machines.Node{Online: true, IP: "100.98.133.20"}, LAN: "192.168.1.20", VMs: []machines.VM{infra}},
	}}
}

func twoMacsServices() services.Fleet {
	public := "jterrazz-macbook-pro.tail77a797.ts.net"
	return services.Fleet{Schema: "station.services/v1", Hosts: []services.HostView{
		{Name: thisMacName, Here: true, Reachable: true, Engine: services.DaemonInfo{Running: true}, PublicHost: public, Serve: "on", HTTPS: "port",
			Services: []services.View{
				{VM: "linux-1", GuestPort: 5173, HostPort: 40001, State: services.Published, Process: "node", HTTP: true, Profile: "vite", URLs: services.URLSet{Public: "https://" + public + ":40001"}},
				{VM: "linux-1", GuestPort: 7000, HostPort: 40002, State: services.Published, Process: "nc"},
			}},
		{Name: miniName, Reachable: true, Engine: services.DaemonInfo{Running: true}},
	}}
}

// value: This Mac's Overview in two-macs: five things need you, the app
// updates first, whose fix is the apps' update.
func twoMacsSummary(updates bool) overview.Summary {
	s := overviewOf(true)
	s.Needs[0] = overview.Need{Section: "apps", Subject: "updates", Text: "2 app updates", Call: "apps.update"}
	s.Lines[1].Parts[1] = overview.Count(catalog.StateDegraded, 2, "updates", "update-available")
	s.Lines[5].Parts = []overview.Part{overview.Count(catalog.StateOK, 8, "", "signed-in"), {State: catalog.StateBroken, Word: "npm needs sign-in"}}
	if !updates {
		s.Needs = s.Needs[1:]
		s.Lines[1].Parts = slices.Delete(slices.Clone(s.Lines[1].Parts), 1, 2)
	}
	return s
}

func thisMacAwake() *awake.Report {
	until := time.Date(2026, 11, 3, 18, 0, 0, 0, time.UTC)
	return &awake.Report{Now: powerNow, Daemon: awake.EngineRunning, Mac: thisMacName, Here: true, Reading: thisMacReading(),
		Session: &awake.Session{Lid: true, StartedAt: powerNow.Add(-time.Hour), Until: until, By: "station awake --until 18:00 --lid"}}
}

func miniAwake() awake.Report {
	return awake.Report{Now: powerNow, Daemon: awake.EngineRunning, Mac: miniName, NeverSleeps: true, NeverHeld: true, NeverSince: powerNow.Add(-12 * 24 * time.Hour), Servers: []string{"infra"}}
}

func twoMacsReading(updates bool) pagemenubar.Reading {
	f, svc, home := twoMacs(), twoMacsServices(), twoMacsSummary(updates)
	return pagemenubar.Reading{Home: &home, Samples: samples(), Fleet: &f, Services: &svc, Power: thisMacAwake(), Macs: []awake.Report{miniAwake()}}
}

func menubarReadings() (frame, quiet, full pagemenubar.Reading) {
	off := powerReport(power.DaemonRunning, nil, thisMacReading())
	off.Daemon, off.Here, off.Mac = awake.EngineRunning, true, thisMacName
	calm := overviewOf(false)
	calm.Lines = calm.Lines[:1]
	calm.VMs = nil
	f := twoMacs()
	alone := machines.Fleet{Hosts: []machines.HostView{f.Hosts[0]}}
	alone.Hosts[0].VMs = nil
	frame = pagemenubar.Reading{Power: &off}
	quiet = pagemenubar.Reading{Home: &calm, Samples: samples(), Fleet: &alone, Power: &off}
	full = twoMacsReading(true)
	return frame, quiet, full
}

func menubarPages() []Fixture {
	frame, quiet, full := menubarReadings()
	return append(states("menubar",
		pagemenubar.Page(frame, menubarHere()),
		pagemenubar.Page(quiet, menubarHere()),
		pagemenubar.Stopped(here(), true),
		pagemenubar.Page(full, menubarHere())), menubarWorlds()...)
}

// value: the menu bar's pages as each world of USE-CASES §0.2 the menu
// bar's proofs cite draws them, named menubar.<page>.<world>.
func menubarWorlds() []Fixture {
	_, quiet, _ := menubarReadings()
	two := twoMacsReading(true)
	down := *two.Fleet
	down.Hosts = slices.Clone(down.Hosts)
	infra := down.Hosts[1].VMs[0]
	infra.State, infra.Detail = machines.Stopped, "restarting"
	infra.Restarts = machines.Restarts{Count: 5, Streak: 5, Looping: true, Wait: time.Minute}
	down.Hosts[1].VMs = []machines.VM{infra}
	infraDown := two
	infraDown.Fleet = &down
	return []Fixture{
		{Name: "menubar.home.one-mac", Page: pagemenubar.Page(quiet, menubarHere())},
		{Name: "menubar.home.two-macs", Page: pagemenubar.Page(two, menubarHere())},
		{Name: "menubar.home.infra-down", Page: pagemenubar.Page(infraDown, menubarHere())},
		{Name: "menubar.home.engine-stopped", Page: pagemenubar.Stopped(menubarHere(), true)},
		{Name: "menubar.switcher.two-macs", Page: pagemenubar.Switcher(two.Fleet, two.Services)},
		{Name: "menubar.vm.linux-1", Page: pagemenubar.MachinePage(linux1Page())},
		{Name: "menubar.vm.infra", Page: pagemenubar.MachinePage(infraPage())},
		{Name: "menubar.mac.mini", Page: pagemenubar.MachinePage(miniPage(false))},
		{Name: "menubar.mac.mini-offline", Page: pagemenubar.MachinePage(miniPage(true))},
	}
}

// value: linux-1's page: its Overview read inside it, GitHub and Claude
// shared from This Mac, its two ports shared.
func linux1Page() pagemenubar.Machine {
	f, svc := twoMacs(), twoMacsServices()
	inside := overview.Summary{Head: overview.Head{Name: "linux-1", Kind: coremachine.Workspace, Up: 2 * time.Hour}, Lines: []overview.Line{
		{Section: "apps", Parts: []overview.Part{overview.Count(catalog.StateOK, 38, "installed", "ok")}},
		{Section: "settings", Parts: []overview.Part{overview.Count(catalog.StateDegraded, 2, "need a look", "attention")}},
		{Section: "disk", Parts: []overview.Part{{Word: "2.4 of 20 GB"}}},
	}}
	lends := accounts.Report{Now: powerNow, Daemon: accounts.DaemonRunning, Machines: []accounts.Machine{{Name: "linux-1", Kind: accounts.KindVM, Lifetime: "throwaway", Running: true, Accounts: []accounts.Account{
		{Provider: catalog.ProviderGitHub, State: catalog.StateOK, Origin: accounts.OriginLent, Scopes: []string{"read only"}, Lent: &accounts.Lend{Name: "github", At: powerNow.Add(-8 * time.Hour), Until: powerNow.Add(4 * time.Hour)}},
		{Provider: catalog.ProviderClaude, State: catalog.StateOK, Origin: accounts.OriginLent, Lent: &accounts.Lend{Name: catalog.ClaudeTokenName, At: powerNow, Until: powerNow.Add(12 * time.Hour)}},
	}}}}
	return pagemenubar.Machine{Asks: Levels, Name: "linux-1", Fleet: &f, Summary: &inside, Samples: readings(4*gb, 20*gb, 17_600_000_000), Services: &svc, Accounts: &lends, Now: powerNow}
}

// value: infra's page from This Mac: up 12 days, its four health steps
// passing, one public port and two private.
func infraPage() pagemenubar.Machine {
	f, svc := twoMacs(), twoMacsServices()
	inside := overview.Summary{Head: overview.Head{Name: "infra", Kind: coremachine.Server, Up: 12*24*time.Hour + 4*time.Hour}, Lines: []overview.Line{
		{Section: "ports", Parts: []overview.Part{{Word: "Public", N: 1, Key: "public", Counted: true}, {Word: "Private", N: 2, Key: "private", Counted: true}}},
		{Section: "ssh", Parts: []overview.Part{{Word: "admin@infra"}}},
	}}
	return pagemenubar.Machine{Asks: Levels, Name: "infra", Fleet: &f, Summary: &inside, Samples: readings(24*gb, 80*gb, 61_000_000_000), Services: &svc, Now: powerNow}
}

// value: mac-mini's page as This Mac reads it from the mini's engine: its
// readiness all applied, infra, and the port that needs you; offline, the
// mini waits at pre-boot for its disk's password.
func miniPage(offline bool) pagemenubar.Machine {
	f, svc := twoMacs(), twoMacsServices()
	ready := func(name, value string) overview.Check {
		return overview.Check{Name: name, State: catalog.StateOK, Value: value}
	}
	s := overview.Summary{Head: overview.Head{Name: miniName, Kind: coremachine.Mac}, Engine: overview.EngineRunning,
		Needs: []overview.Need{{Machine: miniName, Section: "ports", Broken: true, Subject: "5000", Text: "port 5000 is reachable outside Tailscale", Command: "station ports --on mac-mini"}},
		Readiness: []overview.Check{
			ready(machines.ItemRestartsAfterPowerCut, "applied"), ready(machines.ItemNeverSleeps, "applied"), ready(machines.ItemLogsIn, "applied"),
			ready(machines.ItemDiskUnlock, "no password at boot"), ready(machines.ItemPlannedRestarts, "supported"), ready(machines.ItemEngineAtLogin, "installed"),
		}}
	if offline {
		f.Hosts[1].Reachable, f.Hosts[1].Node.Online, f.Hosts[1].Daemon = false, false, machines.DaemonInfo{}
		s.Engine, s.Needs = overview.EngineNone, nil
		s.Readiness[3] = overview.Check{Name: machines.ItemDiskUnlock, State: catalog.StateDegraded, Value: "waits at pre-boot", Detail: "station machines unlock mac-mini (from the same network)"}
	}
	return pagemenubar.Machine{Asks: Levels, Name: miniName, Fleet: &f, Summary: &s, Services: &svc, Now: powerNow}
}

func MenubarStream() []byte {
	// invariant: the recorded stream is written by the daemon's own types from
	// the menubar fixtures: the Swift client reads the bytes a daemon sends.
	byName := map[string]page.Page{}
	for _, f := range All() {
		byName[f.Name] = f.Page
	}
	frame, _ := json.Marshal(page.Document{Schema: page.Schema, Page: byName["menubar.loading"]})
	ready, _ := json.Marshal(page.Document{Schema: page.Schema, Page: byName["menubar.full"]})
	outs := []any{
		engine.Opened{Stream: page.Schema, Pages: []string{engine.PageMenubar}},
		engine.Event{Event: "page", Page: frame},
		engine.Event{Event: "page", Page: ready},
		engine.Event{Event: "tick", At: "2026-11-03T10:30:15Z"},
		engine.Event{Event: "notice", Notice: &engine.Notice{Kind: engine.NoticeEnded, Text: "Power ended: battery 20% on battery"}},
	}
	type answer struct {
		V   int    `json:"v"`
		ID  string `json:"id"`
		Out any    `json:"out"`
	}
	lines := make([]answer, 0, len(outs))
	for _, o := range outs {
		lines = append(lines, answer{V: 2, ID: "1", Out: o})
	}
	var out []byte
	for _, l := range lines {
		data, err := json.Marshal(l)
		if err != nil {
			panic(err)
		}
		out = append(append(out, data...), '\n')
	}
	return out
}

func EventsStream() []byte {
	// invariant: the recorded events watch is written by the engine's own
	// event words, one frame per event of SPEC 10.4 the engine tells, so the
	// menu bar's notifications read what an engine sends.
	var loop machines.Restarts
	at := time.Date(2026, 11, 3, 14, 0, 0, 0, time.UTC)
	for i := range 5 {
		loop = loop.After(time.Second, at.Add(time.Duration(i)*time.Minute))
	}
	exposed := ports.Exposed(ports.Survey([]ports.Machine{ports.MacOf("mac-mini", []ports.Listener{{Port: 5000, Bind: "*", Process: "ControlCenter"}})}, nil))
	low := activity.Sample{DiskFree: 3_000_000_000, DiskTotal: 100_000_000_000}
	events := []engine.Told{
		engine.Restarting("infra", 3), engine.Looping("infra", loop), engine.PoweredOff("infra"), engine.Back("infra"),
		{Kind: engine.NoticeEnded, Parts: []string{engine.AwakeStopped("battery 20 %")}},
		{Kind: engine.EventExposed, Parts: []string{ports.Told(exposed[0])}},
		{Kind: engine.EventDiskLow, Parts: []string{activity.DiskFreeTold("This Mac", low)}},
	}
	outs := make([]any, 0, 1+len(events))
	outs = append(outs, engine.Opened{Stream: engine.EventsStream})
	for _, e := range events {
		outs = append(outs, engine.Event{Event: "notice", Notice: &engine.Notice{Kind: e.Kind, Text: strings.Join(e.Parts, token.Sep)}})
	}
	var out []byte
	for _, o := range outs {
		data, err := json.Marshal(struct {
			V   int    `json:"v"`
			ID  string `json:"id"`
			Out any    `json:"out"`
		}{V: 2, ID: "1", Out: o})
		if err != nil {
			panic(err)
		}
		out = append(append(out, data...), '\n')
	}
	return out
}
