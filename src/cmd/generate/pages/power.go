package pages

import (
	"time"

	"github.com/jterrazz/studio-station/src/awake"

	pagepower "github.com/jterrazz/studio-station/src/pages/power"
)

var powerNow = time.Date(2026, 11, 3, 11, 30, 0, 0, time.UTC)

func thisMacReading() awake.Reading {
	return awake.Reading{Lid: "open", Source: "ac", HasBattery: true, Percent: 80}
}

func powerReport(daemon string, s *awake.Session, r awake.Reading) awake.Report {
	return awake.Report{Now: powerNow, Daemon: daemon, Reading: r, Session: s, Guards: awake.GuardStates(r, s)}
}

func heldSession() *awake.Session {
	start := powerNow.Add(-48 * time.Minute)
	return &awake.Session{Lid: true, StartedAt: start, Until: start.Add(4 * time.Hour), By: "station awake 4h --lid"}
}

func powerPages() []Fixture {
	low := thisMacReading()
	low.Source, low.Percent = "battery", 28
	held := heldSession()
	out := states("power",
		pagepower.Page(powerReport(awake.DaemonPending, nil, thisMacReading()), here()),
		pagepower.Page(powerReport(awake.DaemonRunning, nil, thisMacReading()), here()),
		pagepower.Page(powerReport(awake.DaemonStopped, nil, thisMacReading()), here()),
		pagepower.Page(powerReport(awake.DaemonRunning, held, low), here()),
	)
	awoken := awake.Awoken{Session: *held}
	return append(out, states("outcome-power",
		pending(pagepower.Awake(awoken, nil, here())),
		pagepower.Slept(awake.Slept{}, here()),
		failure("outcome", "awake", "Awake stays off: the battery is at 18 % on battery", "station awake"),
		pagepower.Awake(awoken, nil, here()),
	)...)
}
