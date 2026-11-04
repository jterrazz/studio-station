package pages

import (
	"encoding/json"
	"time"

	"github.com/jterrazz/studio-station/src/catalog"

	"github.com/jterrazz/studio-station/src/activity"
	"github.com/jterrazz/studio-station/src/domain/machines"
	"github.com/jterrazz/studio-station/src/domain/power"
	"github.com/jterrazz/studio-station/src/engine"
	"github.com/jterrazz/studio-station/src/overview"
	"github.com/jterrazz/studio-station/src/page"
	pagemenubar "github.com/jterrazz/studio-station/src/pages/menubar"
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
	return states("menubar",
		pagemenubar.Page(frame, menubarHere()),
		pagemenubar.Page(quiet, menubarHere()),
		pagemenubar.Stopped(here(), true),
		pagemenubar.Page(full, menubarHere()))
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
