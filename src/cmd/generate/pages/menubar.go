package pages

import (
	"encoding/json"
	"github.com/jterrazz/studio-station/src/catalog"
	"time"

	"github.com/jterrazz/studio-station/src/domain/machines"
	"github.com/jterrazz/studio-station/src/domain/power"
	"github.com/jterrazz/studio-station/src/domain/status"
	"github.com/jterrazz/studio-station/src/engine"
	"github.com/jterrazz/studio-station/src/page"
	pagemenubar "github.com/jterrazz/studio-station/src/pages/menubar"
)

func samples() []status.Sample {
	cpu := []float64{12, 18, 25, 34, 30, 22, 41, 55, 48, 36, 28, 31, 44, 38, 29, 34}
	out := make([]status.Sample, 0, len(cpu))
	for i, c := range cpu {
		out = append(out, status.Sample{
			At:     powerNow.Add(time.Duration(i-len(cpu)+1) * 2 * time.Second),
			CPU:    []status.Process{{PID: 812, Name: "station", Value: c}},
			Memory: []status.Process{{PID: 812, Name: catalog.HypervisorTool, Value: 12.4e9}},
		})
	}
	return out
}

func menubarHere() page.Context { return page.Context{Machine: "jterrazz-macbook-pro", Kind: "host"} }

func menubarReadings() (frame, quiet, full pagemenubar.Reading) {
	f, svc := fleet(), serviceFleet()
	held := powerReport(power.DaemonRunning, heldSession(), thisMacReading())
	off := powerReport(power.DaemonRunning, nil, thisMacReading())
	busy, calm := snapshot(), calmSnapshot()
	busy.PowerKnown, busy.Power = true, status.PowerState{Held: true, Lid: true, Until: held.Session.Until}
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
	result, _ := json.Marshal(map[string]any{"stream": page.Schema, "pages": []string{engine.PageMenubar}})
	frame, _ := json.Marshal(page.Document{Schema: page.Schema, Page: byName["menubar.loading"]})
	ready, _ := json.Marshal(page.Document{Schema: page.Schema, Page: byName["menubar.full"]})
	lines := []any{
		engine.Response{V: 1, ID: "1", OK: true, Result: result},
		engine.Event{V: 1, Event: "page", Page: frame},
		engine.Event{V: 1, Event: "page", Page: ready},
		engine.Event{V: 1, Event: "tick", At: "2026-11-03T10:30:15Z"},
		engine.Event{V: 1, Event: "notice", Notice: &engine.Notice{Kind: engine.NoticeEnded, Text: "Power ended: battery 20% on battery"}},
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
