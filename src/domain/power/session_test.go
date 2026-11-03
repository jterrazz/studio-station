package power_test

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/jterrazz/studio-station/src/domain/power"
	"github.com/jterrazz/studio-station/src/system"
	"github.com/jterrazz/studio-station/src/system/systemtest"
)

type port struct {
	r     system.PowerReading
	calls []string
}

func (p *port) HoldIdle(name string) (func(), error) {
	p.calls = append(p.calls, "assert "+name)
	return func() { p.calls = append(p.calls, "release") }, nil
}

func (p *port) SetClamshellSleep(disabled bool) error {
	if disabled {
		p.calls = append(p.calls, "clamshell disabled")
	} else {
		p.calls = append(p.calls, "clamshell normal")
	}
	return nil
}

func (p *port) Read() (system.PowerReading, error) { return p.r, nil }

func (p *port) Watch(context.Context) <-chan system.PowerEvent { return nil }

func (p *port) count(call string) int {
	n := 0
	for _, c := range p.calls {
		if c == call {
			n++
		}
	}
	return n
}

type world struct {
	t       *testing.T
	port    *port
	clock   *systemtest.Clock
	run     *systemtest.Runner
	m       *power.Machine
	notices []power.End
}

func laptop() system.PowerReading {
	return system.PowerReading{Lid: "open", Source: "ac", HasBattery: true, Percent: 80}
}

func newWorld(t *testing.T, r system.PowerReading) *world {
	w := &world{t: t, port: &port{r: r}, clock: &systemtest.Clock{At: time.Date(2026, 11, 3, 10, 0, 0, 0, time.UTC)}, run: systemtest.NewRunner(t)}
	w.run.On("/usr/bin/pmset", "displaysleepnow")
	w.m = power.New(w.port, w.run, w.clock, power.Options{Pmset: "/usr/bin/pmset", Notice: func(e power.End) { w.notices = append(w.notices, e) }})
	return w
}

func (w *world) awake(req power.Request) {
	w.t.Helper()
	if req.By == "" {
		req.By = "station power awake"
	}
	if _, _, err := w.m.Awake(req); err != nil {
		w.t.Fatalf("awake: %v", err)
	}
}

func (w *world) pass(d time.Duration) {
	for step := time.Duration(0); step < d; step += time.Second {
		w.clock.At = w.clock.At.Add(time.Second)
		w.m.Tick()
	}
}

func (w *world) event(kind string) { w.m.Observe(context.Background(), system.PowerEvent{Kind: kind}) }

func (w *world) ended(reason string) {
	w.t.Helper()
	v := w.m.View()
	if v.Session != nil {
		w.t.Fatalf("the session still holds; want it ended by %s", reason)
	}
	if v.LastEnd == nil || v.LastEnd.Reason != reason {
		w.t.Fatalf("last end = %+v, want reason %s", v.LastEnd, reason)
	}
	if last := w.port.calls[len(w.port.calls)-1]; last != "clamshell normal" {
		w.t.Fatalf("the last call is %q, want the clamshell reset to normal: %v", last, w.port.calls)
	}
	if w.port.count("release") != 1 {
		w.t.Fatalf("the assertion was released %d times: %v", w.port.count("release"), w.port.calls)
	}
}

func (w *world) held() {
	w.t.Helper()
	if !w.m.Held() {
		w.t.Fatalf("the session ended (%+v); want it held", w.m.View().LastEnd)
	}
}

func TestEachGuardPastItsThresholdRefusesTheStart(t *testing.T) {
	for name, c := range map[string]struct {
		r     func(*system.PowerReading)
		req   power.Request
		guard string
	}{
		"battery":   {func(r *system.PowerReading) { r.Source, r.Percent = "battery", 18 }, power.Request{}, power.ReasonBattery},
		"low power": {func(r *system.PowerReading) { r.LowPower = true }, power.Request{}, power.ReasonLowPower},
		"thermal":   {func(r *system.PowerReading) { r.Thermal = 3 }, power.Request{}, power.ReasonThermal},
		"lid":       {func(r *system.PowerReading) { r.Source, r.Lid = "battery", "closed" }, power.Request{Lid: true}, power.ReasonLid},
	} {
		t.Run(name, func(t *testing.T) {
			r := laptop()
			c.r(&r)
			w := newWorld(t, r)
			_, _, err := w.m.Awake(c.req)
			var g *power.GuardError
			if !errors.As(err, &g) || g.Guard != c.guard {
				t.Fatalf("awake = %v, want the %s guard's refusal", err, c.guard)
			}
			if len(w.port.calls) != 0 {
				t.Fatalf("a refused start called the port: %v", w.port.calls)
			}
		})
	}
}

func TestBatteryLetsTheLidCloseOnBattery(t *testing.T) {
	r := laptop()
	r.Source, r.Lid = "battery", "closed"
	w := newWorld(t, r)
	w.awake(power.Request{Lid: true, Battery: true})
	w.event("lid")
	w.held()
	if g := power.Guards(w.m.View().Reading, w.m.View().Session); g[3].State != power.GuardNear {
		t.Fatalf("the lid guard reads %+v, want near while allowed", g[3])
	}
}

func TestARefreshKeepsOneAssertionAndOnlyTheLidTouchesTheClamshell(t *testing.T) {
	w := newWorld(t, laptop())
	w.awake(power.Request{})
	if w.port.count("clamshell disabled") != 0 {
		t.Fatalf("a session without --lid disabled the clamshell: %v", w.port.calls)
	}
	_, refreshed, err := w.m.Awake(power.Request{Lid: true, For: time.Hour, By: "station power awake --lid --for 1h"})
	if err != nil || !refreshed {
		t.Fatalf("the second awake = %v, refreshed %v", err, refreshed)
	}
	if w.port.count("assert station: station power awake") != 1 || w.port.count("clamshell disabled") != 1 {
		t.Fatalf("a refresh took a second assertion or no clamshell: %v", w.port.calls)
	}
	if s := w.m.View().Session; !s.Lid || s.Until.Sub(w.clock.At) != time.Hour {
		t.Fatalf("the refreshed session is %+v", s)
	}
}

func TestEveryEndResetsTheClamshellAndRecordsWhy(t *testing.T) {
	for reason, end := range map[string]func(w *world){
		power.ReasonUntil: func(w *world) { w.pass(time.Hour) },
		power.ReasonSleep: func(w *world) {
			if s := w.m.Sleep(); !s.Ended || s.After != 0 {
				t.Fatalf("sleep = %+v", s)
			}
		},
		power.ReasonStop: func(w *world) { w.m.Stop() },
		power.ReasonQuit: func(w *world) { w.m.Quit() },
		power.ReasonBattery: func(w *world) {
			w.port.r.Source, w.port.r.Percent = "battery", 20
			w.event("source")
		},
		power.ReasonLowPower: func(w *world) {
			w.port.r.LowPower = true
			w.pass(29 * time.Second)
			w.held()
			w.pass(time.Second)
		},
		power.ReasonThermal: func(w *world) {
			w.port.r.Thermal = 3
			w.event("thermal")
		},
	} {
		t.Run(reason, func(t *testing.T) {
			w := newWorld(t, laptop())
			w.awake(power.Request{Lid: true, Battery: true, For: time.Hour})
			end(w)
			w.ended(reason)
			if notified := len(w.notices) == 1; notified == (reason == power.ReasonSleep) {
				t.Fatalf("notices %v after %s: power sleep alone ends silently", w.notices, reason)
			}
		})
	}
}

func TestHeavyThermalEndsOnlyAfterFiveContinuousMinutesLidClosedOnBattery(t *testing.T) {
	r := laptop()
	r.Lid, r.Source, r.Thermal = "closed", "battery", 2
	w := newWorld(t, r)
	w.awake(power.Request{Lid: true, Battery: true})
	w.event("thermal")
	w.pass(4*time.Minute + 59*time.Second)
	w.held()
	w.port.r.Lid = "open"
	w.event("lid")
	w.port.r.Lid = "closed"
	w.event("lid")
	w.pass(4*time.Minute + 59*time.Second)
	w.held()
	w.pass(31 * time.Second)
	w.ended(power.ReasonThermal)
}

func TestHeavyThermalOnACNeverEnds(t *testing.T) {
	r := laptop()
	r.Lid, r.Thermal = "closed", 2
	w := newWorld(t, r)
	w.awake(power.Request{Lid: true, For: time.Hour})
	w.pass(30 * time.Minute)
	w.held()
}

func TestTheLidGuardTripsInEitherOrder(t *testing.T) {
	for name, order := range map[string][2]func(*system.PowerReading){
		"lid then battery": {func(r *system.PowerReading) { r.Lid = "closed" }, func(r *system.PowerReading) { r.Source = "battery" }},
		"battery then lid": {func(r *system.PowerReading) { r.Source = "battery" }, func(r *system.PowerReading) { r.Lid = "closed" }},
	} {
		t.Run(name, func(t *testing.T) {
			w := newWorld(t, laptop())
			w.awake(power.Request{Lid: true})
			order[0](&w.port.r)
			w.event("lid")
			w.held()
			order[1](&w.port.r)
			w.event("source")
			w.ended(power.ReasonLid)
		})
	}
}

func TestTheMaskIsSetAgainOnEverySourceAndLidChangeWhileTheLidHolds(t *testing.T) {
	w := newWorld(t, laptop())
	w.awake(power.Request{Lid: true, Battery: true})
	for _, kind := range []string{"source", "lid", "thermal", "lid"} {
		w.event(kind)
	}
	if n := w.port.count("clamshell disabled"); n != 4 {
		t.Fatalf("the clamshell was disabled %d times, want 1 at the start and 3 again: %v", n, w.port.calls)
	}
}

func TestTheDisplaySleepsOnLidCloseUnderTheLidOnly(t *testing.T) {
	for name, lid := range map[string]bool{"--lid": true, "without --lid": false} {
		t.Run(name, func(t *testing.T) {
			w := newWorld(t, laptop())
			w.awake(power.Request{Lid: lid})
			w.port.r.Lid = "closed"
			w.event("lid")
			ran := slices.ContainsFunc(w.run.Calls, func(c system.Cmd) bool { return c.Name == "/usr/bin/pmset" })
			if ran != lid {
				t.Fatalf("pmset ran = %v, want %v: %v", ran, lid, w.run.Calls)
			}
		})
	}
}

func TestAHeldSessionLogsItsHeartbeatEvery30Seconds(t *testing.T) {
	w := newWorld(t, laptop())
	var beats []string
	w.m = power.New(w.port, w.run, w.clock, power.Options{Log: func(line string) {
		if strings.HasPrefix(line, "held ") {
			beats = append(beats, line)
		}
	}})
	w.awake(power.Request{})
	w.pass(65 * time.Second)
	want := []string{"held lid=open source=ac battery=80% thermal=0", "held lid=open source=ac battery=80% thermal=0"}
	if !slices.Equal(beats, want) {
		t.Fatalf("heartbeats over 65s = %q, want %q", beats, want)
	}
	w.m.Sleep()
	w.pass(time.Minute)
	if len(beats) != 2 {
		t.Fatalf("a heartbeat was logged with no session held: %q", beats)
	}
}
