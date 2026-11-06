package pages

import (
	"time"

	"github.com/jterrazz/studio-station/src/domain/power"
	pagekeepawake "github.com/jterrazz/studio-station/src/pages/keepawake"
)

var keepAwakeNow = time.Date(2026, 11, 3, 11, 30, 0, 0, time.UTC)

func thisMacReading() power.Reading {
	return power.Reading{Lid: "open", Source: "ac", HasBattery: true, Percent: 80}
}

func keepAwakeReport(engine string, s *power.Session, r power.Reading) power.Report {
	return power.Report{Now: keepAwakeNow, Engine: engine, Reading: r, Session: s, Guards: power.Guards(r, s)}
}

func heldSession() *power.Session {
	start := keepAwakeNow.Add(-48 * time.Minute)
	return &power.Session{Lid: true, StartedAt: start, Until: start.Add(4 * time.Hour), By: "station awake 4h --lid"}
}

func keepAwakePages() []Fixture {
	low := thisMacReading()
	low.Source, low.Percent = "battery", 28
	held := heldSession()
	out := states("keep-awake",
		pagekeepawake.Page(keepAwakeReport(power.DaemonPending, nil, thisMacReading()), thisMacContext()),
		pagekeepawake.Page(keepAwakeReport(power.DaemonRunning, nil, thisMacReading()), thisMacContext()),
		pagekeepawake.Page(keepAwakeReport(power.DaemonStopped, nil, thisMacReading()), thisMacContext()),
		pagekeepawake.Page(keepAwakeReport(power.DaemonRunning, held, low), thisMacContext()),
	)
	awoken := power.Awoken{Session: *held}
	return append(out, states("outcome-keep-awake",
		pending(pagekeepawake.Awake(awoken, nil, thisMacContext())),
		pagekeepawake.Slept(power.Slept{}, thisMacContext()),
		failure("outcome", "awake", "Awake stays off: the battery is at 18 % on battery", "station awake"),
		pagekeepawake.Awake(awoken, nil, thisMacContext()),
	)...)
}
