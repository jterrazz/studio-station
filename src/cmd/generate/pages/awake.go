package pages

import (
	"time"

	"github.com/jterrazz/studio-station/src/awake"
	"github.com/jterrazz/studio-station/src/core/machine"
	pageawake "github.com/jterrazz/studio-station/src/pages/awake"
)

// value: Awake on every Mac: This Mac a laptop kept awake with its lid
// closed, the mini a desktop that never sleeps and hosts infra.
func awakeFleet(engine string, held bool) pageawake.Fleet {
	laptop := awake.Report{Now: powerNow, Daemon: engine, Mac: "jterrazz-macbook-pro", Here: true, Reading: thisMacReading(),
		Caps: machine.Capabilities{Battery: true, Lid: true, Thermal: true, Engine: true}}
	mini := awake.Report{Now: powerNow, Daemon: engine, Mac: "mac-mini", Reading: awake.Reading{Source: "ac"},
		Caps: machine.Capabilities{Thermal: true, Engine: true}, Servers: []string{"infra"}}
	if held {
		laptop.Session = heldSession()
		mini.NeverSleeps, mini.NeverHeld, mini.NeverSince = true, true, powerNow.Add(-12*24*time.Hour)
	}
	return pageawake.Fleet{Now: powerNow, Reports: []awake.Report{laptop, mini}}
}

func awakePages() []Fixture {
	return states("awake",
		pageawake.Page(awakeFleet(awake.EngineReading, false), here()),
		pageawake.Page(awakeFleet(awake.EngineRunning, false), here()),
		failure("awake", "awake", "the engine is stopped on jterrazz-macbook-pro", "station engine start"),
		pageawake.Page(awakeFleet(awake.EngineRunning, true), here()),
	)
}
