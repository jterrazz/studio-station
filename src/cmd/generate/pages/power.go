package pages

import (
	"time"

	"github.com/jterrazz/studio-station/src/domain/power"
	pagepower "github.com/jterrazz/studio-station/src/pages/power"
)

var powerNow = time.Date(2026, 11, 3, 11, 30, 0, 0, time.UTC)

func thisMacReading() power.Reading {
	return power.Reading{Lid: "open", Source: "ac", HasBattery: true, Percent: 80}
}

func powerReport(engine string, s *power.Session, r power.Reading) power.Report {
	return power.Report{Now: powerNow, Daemon: engine, Reading: r, Session: s, Guards: power.Guards(r, s)}
}

func heldSession() *power.Session {
	start := powerNow.Add(-48 * time.Minute)
	return &power.Session{Lid: true, StartedAt: start, Until: start.Add(4 * time.Hour), By: "station awake 4h --lid"}
}

func powerPages() []Fixture {
	low := thisMacReading()
	low.Source, low.Percent = "battery", 28
	held := heldSession()
	out := states("power",
		pagepower.Page(powerReport(power.DaemonPending, nil, thisMacReading()), here()),
		pagepower.Page(powerReport(power.DaemonRunning, nil, thisMacReading()), here()),
		pagepower.Page(powerReport(power.DaemonStopped, nil, thisMacReading()), here()),
		pagepower.Page(powerReport(power.DaemonRunning, held, low), here()),
	)
	awoken := power.Awoken{Session: *held}
	return append(out, states("outcome-power",
		pending(pagepower.Awake(awoken, nil, here())),
		pagepower.Slept(power.Slept{}, here()),
		failure("outcome", "awake", "Awake stays off: the battery is at 18 % on battery", "station awake"),
		pagepower.Awake(awoken, nil, here()),
	)...)
}
