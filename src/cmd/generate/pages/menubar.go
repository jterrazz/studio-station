package pages

import (
	"encoding/json"
	"slices"
	"strings"
	"time"

	"github.com/jterrazz/studio-station/src/catalog"

	"github.com/jterrazz/studio-station/src/activity"
	"github.com/jterrazz/studio-station/src/domain/machines"
	"github.com/jterrazz/studio-station/src/domain/power"
	"github.com/jterrazz/studio-station/src/engine"
	"github.com/jterrazz/studio-station/src/overview"
	"github.com/jterrazz/studio-station/src/page"
	pagehome "github.com/jterrazz/studio-station/src/pages/home"
	pagemenubar "github.com/jterrazz/studio-station/src/pages/menubar"
	"github.com/jterrazz/studio-station/src/ports"
	"github.com/jterrazz/studio-station/src/ui/token"
)

func samples() []activity.Reading {
	cpu := []float64{12, 18, 25, 34, 30, 22, 41, 55, 48, 36, 28, 31, 44, 38, 29, 34}
	top := activity.Busiest{
		CPU:    []activity.Process{{PID: 812, Name: "station", Value: cpu[len(cpu)-1]}},
		Memory: []activity.Process{{PID: 812, Name: catalog.HypervisorTool, Value: 12.4e9}},
	}
	at := func(i int) time.Time { return powerNow.Add(time.Duration(i-len(cpu)) * 2 * time.Second) }
	out := make([]activity.Reading, 0, len(cpu)+1)
	out = append(out, activity.Reading{Sample: activity.Sample{At: at(0), CPUTotal: 1000, MemUsed: 12.4e9}, Top: top})
	for i, c := range cpu {
		prev := out[len(out)-1].Sample
		out = append(out, activity.Reading{Sample: activity.Sample{
			At: at(i + 1), CPUBusy: prev.CPUBusy + uint64(c*10), CPUTotal: prev.CPUTotal + 1000, MemUsed: 12.4e9,
		}, Top: top})
	}
	return out
}

func menubarHere() page.Context { return page.Context{Machine: "jterrazz-macbook-pro", Kind: "host"} }

func menubarReadings() (frame, quiet, full pagemenubar.Reading) {
	f, svc := fleet(), serviceFleet()
	held := powerReport(power.DaemonRunning, heldSession(), thisMacReading())
	off := powerReport(power.DaemonRunning, nil, thisMacReading())
	busy, calm := snapshot(), calmSnapshot()
	busy.PowerKnown, busy.Power = true, overview.PowerState{Held: true, Lid: true, Until: held.Session.Until}
	alone := machines.Fleet{Hosts: []machines.HostView{f.Hosts[0]}}
	alone.Hosts[0].VMs = nil
	frame = pagemenubar.Reading{Power: &off}
	quiet = pagemenubar.Reading{Home: &calm, Samples: samples(), Fleet: &alone, Power: &off}
	full = pagemenubar.Reading{Home: &busy, Samples: samples(), Fleet: &f, Services: &svc, Power: &held}
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

// value: This Mac in the two-macs world: its engine runs, and five things
// need you (MB-02).
func twoMacsHome() pagehome.Overview {
	s := snapshot()
	s.PowerKnown, s.Power = true, overview.PowerState{Held: true, Lid: true, Until: heldSession().Until}
	s.Station.Daemon = check("daemon", catalog.StateOK, "running v2.0.0-250", "")
	s.Environment.Updates[0] = check("macos", catalog.StateOK, "up to date", "checked 22m ago")
	s.Config = s.Config[:1]
	s.Background = s.Background[:2]
	return s
}

// value: the menu bar's Home as each world of USE-CASES §0.2 the menu bar's
// proofs cite draws it, named menubar.home.<world>.
func menubarWorlds() []Fixture {
	_, quiet, full := menubarReadings()
	f := *full.Fleet
	f.Hosts = slices.Clone(f.Hosts)
	f.Hosts[1].VMs = []machines.VM{server().VM}
	two := full
	home := twoMacsHome()
	two.Home, two.Fleet = &home, &f
	down := f
	down.Hosts = slices.Clone(f.Hosts)
	infra := server().VM
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
	}
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
