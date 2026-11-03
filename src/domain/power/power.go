package power

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jterrazz/studio-station/src/system"
)

const (
	Default    = 4 * time.Hour
	Longest    = 24 * time.Hour
	Shortest   = time.Minute
	pollEvery  = 30 * time.Second
	heavyLimit = 5 * time.Minute
	keepEnd    = 24 * time.Hour
	pmsetWait  = 5 * time.Second

	batteryEnds  = 20
	batteryWarns = 30
	thermalHeavy = 2
	thermalEnds  = 3
)

const (
	ReasonThermal  = "thermal"
	ReasonBattery  = "battery"
	ReasonLowPower = "low_power"
	ReasonLid      = "lid"
	ReasonUntil    = "until"
	ReasonSleep    = "sleep"
	ReasonStop     = "stop"
	ReasonQuit     = "quit"
)

const (
	GuardOK   = "ok"
	GuardNear = "near"
)

type Reading = system.PowerReading

type Request struct {
	Lid     bool
	Battery bool
	For     time.Duration
	By      string
}

type Session struct {
	Lid       bool      `json:"lid"`
	Battery   bool      `json:"battery"`
	StartedAt time.Time `json:"started_at"`
	Until     time.Time `json:"until"`
	By        string    `json:"by"`
}

type End struct {
	At     time.Time `json:"at"`
	Reason string    `json:"reason"`
	Detail string    `json:"detail"`
}

type Guard struct {
	Name  string `json:"name"`
	State string `json:"state"`
}

type View struct {
	Session *Session `json:"session"`
	LastEnd *End     `json:"last_end"`
	Reading Reading  `json:"reading"`
	Guards  []Guard  `json:"guards"`
}

type Slept struct {
	Ended bool          `json:"ended"`
	After time.Duration `json:"after"`
}

type RangeError struct{ Long bool }

func (e *RangeError) Error() string {
	if e.Long {
		return "a Power session lasts at most 24h"
	}
	return "a Power session lasts at least 1m"
}

type GuardError struct {
	Guard string
	Words string
}

func (e *GuardError) Error() string { return "Power stays off: " + e.Words }

type UnsupportedError struct{}

func (*UnsupportedError) Error() string { return "Power runs on a macOS host" }

func CheckFor(d time.Duration) error {
	switch {
	case d > Longest:
		return &RangeError{Long: true}
	case d < Shortest:
		return &RangeError{}
	}
	return nil
}

func Level(n int) string {
	switch n {
	case 0:
		return "nominal"
	case 1:
		return "moderate"
	case thermalHeavy:
		return "heavy"
	case thermalEnds:
		return "trapping"
	}
	return "sleeping"
}

func onBattery(r Reading) bool { return r.HasBattery && r.Source == "battery" }

func lidOnBattery(r Reading) bool { return r.Lid == "closed" && onBattery(r) }

// invariant: the first guard past its threshold names the end; a desktop
// Mac (no battery) never trips the battery guard nor the lid's battery half.
func trip(r Reading, s *Session, heavyFor time.Duration) (reason, detail, words string) {
	switch {
	case r.Thermal >= thermalEnds:
		return ReasonThermal, "thermal " + Level(r.Thermal), "the Mac is under serious thermal pressure"
	case r.Thermal == thermalHeavy && heavyFor >= heavyLimit:
		return ReasonThermal, "thermal heavy for 5m lid closed on battery", "the Mac is under heavy thermal pressure"
	case onBattery(r) && r.Percent <= batteryEnds:
		p := strconv.Itoa(r.Percent) + "%"
		return ReasonBattery, "battery " + p + " on battery", "the battery is at " + p + " on battery"
	case r.LowPower:
		return ReasonLowPower, "Low Power Mode on", "Low Power Mode is on"
	case s != nil && s.Lid && !s.Battery && lidOnBattery(r):
		return ReasonLid, "lid closed on battery", "the lid is closed on battery"
	}
	return "", "", ""
}

func Guards(r Reading, s *Session) []Guard {
	state := func(near bool) string {
		if near {
			return GuardNear
		}
		return GuardOK
	}
	return []Guard{
		{Name: ReasonBattery, State: state(onBattery(r) && r.Percent <= batteryWarns)},
		{Name: ReasonLowPower, State: state(false)},
		{Name: ReasonThermal, State: state(r.Thermal == thermalHeavy)},
		{Name: ReasonLid, State: state(s != nil && s.Lid && s.Battery && lidOnBattery(r))},
	}
}

type Options struct {
	// value: the absolute pmset a lid close runs displaysleepnow with.
	Pmset string
	Log   func(message string)
	// value: a session a guard, its term or a stop ended; never power sleep.
	Notice func(End)
	Change func()
}

type Machine struct {
	port  system.Power
	run   system.Runner
	clock system.Clock
	o     Options

	mu         sync.Mutex
	session    *Session
	release    func()
	lastEnd    *End
	reading    Reading
	heavySince time.Time
	polled     time.Time
	unreset    bool
}

type absent struct{}

func (absent) HoldIdle(string) (func(), error) { return nil, system.ErrPowerUnsupported }
func (absent) SetClamshellSleep(bool) error    { return system.ErrPowerUnsupported }
func (absent) Read() (system.PowerReading, error) {
	return system.PowerReading{}, system.ErrPowerUnsupported
}
func (absent) Watch(context.Context) <-chan system.PowerEvent {
	out := make(chan system.PowerEvent)
	close(out)
	return out
}

func New(port system.Power, run system.Runner, clock system.Clock, o Options) *Machine {
	if port == nil {
		port = absent{}
	}
	return &Machine{port: port, run: run, clock: clock, o: o}
}

func (m *Machine) log(message string) {
	if m.o.Log != nil {
		m.o.Log(message)
	}
}

func (m *Machine) changed() {
	if m.o.Change != nil {
		m.o.Change()
	}
}

func unsupported(err error) bool { return errors.Is(err, system.ErrPowerUnsupported) }

// invariant: resetting the clamshell is the safe direction, so a reset that
// fails is retried at every poll until it passes; a Mac with no power
// management has nothing to reset.
func (m *Machine) resetLocked() {
	err := m.port.SetClamshellSleep(false)
	switch {
	case err == nil:
		m.unreset = false
	case unsupported(err):
		m.unreset = false
	default:
		m.unreset = true
		m.log("reset failed, retried in 30s: " + err.Error())
	}
}

func (m *Machine) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.resetLocked()
}

func (m *Machine) Held() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.session != nil
}

func (m *Machine) View() View {
	r, _ := m.port.Read()
	m.mu.Lock()
	defer m.mu.Unlock()
	m.reading = r
	v := View{Reading: r, Guards: Guards(r, m.session)}
	if m.session != nil {
		s := *m.session
		v.Session = &s
	}
	if m.lastEnd != nil && m.clock.Now().Sub(m.lastEnd.At) < keepEnd {
		e := *m.lastEnd
		v.LastEnd = &e
	}
	return v
}

func flags(s Session) string {
	var on []string
	if s.Lid {
		on = append(on, "lid")
	}
	if s.Battery {
		on = append(on, "battery")
	}
	if len(on) == 0 {
		return ""
	}
	return " (" + strings.Join(on, ", ") + ")"
}

func (m *Machine) Awake(req Request) (Session, bool, error) {
	if req.For == 0 {
		req.For = Default
	}
	if err := CheckFor(req.For); err != nil {
		return Session{}, false, err
	}
	r, err := m.port.Read()
	if unsupported(err) {
		return Session{}, false, &UnsupportedError{}
	}
	if err != nil {
		return Session{}, false, err
	}
	want := Session{Lid: req.Lid, Battery: req.Battery, By: req.By}
	if reason, _, words := trip(r, &want, 0); reason != "" {
		return Session{}, false, &GuardError{Guard: reason, Words: words}
	}
	m.mu.Lock()
	defer m.changed()
	defer m.mu.Unlock()
	m.reading = r
	now := m.clock.Now().UTC().Truncate(time.Second)
	want.StartedAt, want.Until = now, now.Add(req.For)
	refreshed := m.session != nil
	if refreshed {
		was := m.session.Lid
		want.StartedAt = m.session.StartedAt
		m.session = &want
		if want.Lid != was {
			if err := m.port.SetClamshellSleep(want.Lid); err != nil {
				m.log("clamshell: " + err.Error())
			}
		}
	} else {
		release, err := m.port.HoldIdle("station: " + req.By)
		if err != nil {
			if unsupported(err) {
				return Session{}, false, &UnsupportedError{}
			}
			return Session{}, false, err
		}
		m.release, m.session = release, &want
		m.heavySince, m.polled = time.Time{}, now
		if want.Lid {
			if err := m.port.SetClamshellSleep(true); err != nil {
				m.log("clamshell: " + err.Error())
			}
		}
	}
	m.log("awake until " + want.Until.Format(time.RFC3339) + flags(want) + " by " + want.By)
	return want, refreshed, nil
}

// invariant: every end releases the assertion and sets the clamshell back
// to normal, --lid or not, so whatever way a session ends the Mac sleeps as
// macOS decides.
func (m *Machine) endLocked(reason, detail string) (End, time.Duration) {
	now := m.clock.Now().UTC().Truncate(time.Second)
	after := now.Sub(m.session.StartedAt)
	if m.release != nil {
		m.release()
	}
	m.release, m.session, m.heavySince = nil, nil, time.Time{}
	m.resetLocked()
	e := End{At: now, Reason: reason, Detail: detail}
	m.lastEnd = &e
	m.log("ended (" + reason + ": " + detail + ")")
	return e, after
}

func (m *Machine) finish(reason, detail string) bool {
	m.mu.Lock()
	if m.session == nil {
		m.mu.Unlock()
		return false
	}
	e, _ := m.endLocked(reason, detail)
	m.mu.Unlock()
	if reason != ReasonSleep && m.o.Notice != nil {
		m.o.Notice(e)
	}
	m.changed()
	return true
}

func (m *Machine) Sleep() Slept {
	m.mu.Lock()
	if m.session == nil {
		m.mu.Unlock()
		return Slept{}
	}
	_, after := m.endLocked(ReasonSleep, "station power sleep")
	m.mu.Unlock()
	m.changed()
	return Slept{Ended: true, After: after}
}

func (m *Machine) Stop() bool { return m.finish(ReasonStop, "station stop") }

func (m *Machine) Quit() {
	if !m.finish(ReasonQuit, "the daemon stopped") {
		m.Reset()
	}
}

func term(d time.Duration) string {
	h, mins := int(d.Hours()), int(d.Minutes())%60
	switch {
	case h > 0 && mins > 0:
		return strconv.Itoa(h) + "h " + strconv.Itoa(mins) + "m"
	case h > 0:
		return strconv.Itoa(h) + "h"
	}
	return strconv.Itoa(mins) + "m"
}

func (m *Machine) evaluateLocked(r Reading) (reason, detail string) {
	now := m.clock.Now()
	m.reading = r
	if m.session == nil {
		return "", ""
	}
	if r.Thermal == thermalHeavy && lidOnBattery(r) {
		if m.heavySince.IsZero() {
			m.heavySince = now
		}
	} else {
		m.heavySince = time.Time{}
	}
	heavyFor := time.Duration(0)
	if !m.heavySince.IsZero() {
		heavyFor = now.Sub(m.heavySince)
	}
	reason, detail, _ = trip(r, m.session, heavyFor)
	return reason, detail
}

func (m *Machine) Observe(ctx context.Context, ev system.PowerEvent) {
	r, err := m.port.Read()
	if err != nil {
		return
	}
	m.mu.Lock()
	reason, detail := m.evaluateLocked(r)
	if reason != "" {
		m.mu.Unlock()
		m.finish(reason, detail)
		return
	}
	lidHeld := m.session != nil && m.session.Lid
	if lidHeld && (ev.Kind == "source" || ev.Kind == "lid") {
		// invariant: powerd may clear the mask (the safe direction), so it
		// is set again on every source and lid change while --lid holds.
		if err := m.port.SetClamshellSleep(true); err != nil {
			m.log("clamshell: " + err.Error())
		}
	}
	m.mu.Unlock()
	if lidHeld && ev.Kind == "lid" && r.Lid == "closed" {
		m.sleepDisplay(ctx)
	}
	m.changed()
}

// invariant: the display sleeps by pmset displaysleepnow alone, never a
// display assertion, so no other app's assertion keeps it lit lid closed.
func (m *Machine) sleepDisplay(ctx context.Context) {
	out, err := m.run.Output(ctx, system.Cmd{Name: m.o.Pmset, Args: []string{"displaysleepnow"}, Timeout: pmsetWait})
	switch {
	case err != nil:
		m.log("display: " + err.Error())
	case out.ExitCode != 0:
		m.log("display: pmset exited " + strconv.Itoa(out.ExitCode))
	}
}

func (m *Machine) Tick() {
	// value: called every second by the daemon: the term is checked each time;
	// the guards, Low Power Mode's poll, the heartbeat and a pending reset every
	// 30 s.
	now := m.clock.Now()
	m.mu.Lock()
	if m.session != nil && !now.Before(m.session.Until) {
		detail := "its " + term(m.session.Until.Sub(m.session.StartedAt)) + " ended"
		m.mu.Unlock()
		m.finish(ReasonUntil, detail)
		return
	}
	due := now.Sub(m.polled) >= pollEvery
	if !due {
		m.mu.Unlock()
		return
	}
	m.polled = now
	if m.unreset && m.session == nil {
		m.resetLocked()
	}
	if m.session == nil {
		m.mu.Unlock()
		return
	}
	m.mu.Unlock()
	r, err := m.port.Read()
	if err != nil {
		return
	}
	m.mu.Lock()
	reason, detail := m.evaluateLocked(r)
	held := m.session != nil
	m.mu.Unlock()
	if reason != "" {
		m.finish(reason, detail)
		return
	}
	if held {
		m.log("held " + heartbeat(r))
		m.changed()
	}
}

func heartbeat(r Reading) string {
	lid := r.Lid
	if lid == "" {
		lid = "none"
	}
	battery := "none"
	if r.HasBattery {
		battery = strconv.Itoa(r.Percent) + "%"
	}
	return "lid=" + lid + " source=" + r.Source + " battery=" + battery + " thermal=" + strconv.Itoa(r.Thermal)
}
