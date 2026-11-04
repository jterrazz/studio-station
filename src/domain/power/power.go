package power

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jterrazz/studio-station/src/awake"
	"github.com/jterrazz/studio-station/src/awake/contract"
	"github.com/jterrazz/studio-station/src/core/machine"
	"github.com/jterrazz/studio-station/src/system"
)

const (
	Default   = awake.DefaultFor
	Longest   = 24 * time.Hour
	Shortest  = time.Minute
	pollEvery = 30 * time.Second
	keepEnd   = 24 * time.Hour
)

const (
	ReasonThermal  = awake.Thermal
	ReasonBattery  = awake.Battery
	ReasonLowPower = awake.LowPower
	ReasonLid      = awake.Lid
	ReasonUntil    = "until"
	ReasonSleep    = "sleep"
	ReasonStop     = "stop"
	ReasonQuit     = "quit"
)

const (
	GuardOK   = "ok"
	GuardNear = "near"
)

type Reading = awake.Reading

type Port = contract.Power

type (
	Request = awake.Request
	Session = awake.Session
	End     = awake.End
	Guard   = awake.GuardState
	View    = awake.View
	Slept   = awake.Slept
)

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

func Level(n int) string { return awake.Level(n) }

// invariant: the first active guard past its threshold names the end; a
// guard is active only when this Mac reports the capability it reads.
func trip(c machine.Capabilities, r Reading, s *Session, heavyFor time.Duration) (reason, detail, words string) {
	held := awake.Held{}
	if s != nil {
		held = awake.Held{Lid: s.Lid, Battery: s.Battery}
	}
	t, ok := awake.Check(c, r, held, heavyFor)
	if !ok {
		return "", "", ""
	}
	return t.Guard, t.Detail, t.Words
}

func Guards(r Reading, s *Session) []Guard {
	state := func(near bool) string {
		if near {
			return GuardNear
		}
		return GuardOK
	}
	return []Guard{
		{Name: ReasonBattery, State: state(r.OnBattery() && r.Percent <= awake.BatteryWarns)},
		{Name: ReasonLowPower, State: state(false)},
		{Name: ReasonThermal, State: state(r.Thermal == awake.ThermalHeavy)},
		{Name: ReasonLid, State: state(s != nil && s.Lid && s.Battery && r.LidClosedOnBattery())},
	}
}

type Options struct {
	Log func(message string)
	// value: a session a guard, its term or a stop ended; never power sleep.
	Notice func(End)
	Change func()
}

type Machine struct {
	port  Port
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

func (absent) Capabilities() contract.PowerCaps { return contract.PowerCaps{} }
func (absent) Read() (awake.Reading, error)     { return awake.Reading{}, contract.ErrUnsupported }
func (absent) Hold(string) (func(), error)      { return nil, contract.ErrUnsupported }
func (absent) Clamshell(bool) error             { return contract.ErrUnsupported }
func (absent) DisplaySleepNow(context.Context) error {
	return contract.ErrUnsupported
}
func (absent) Watch(context.Context) <-chan awake.Event {
	out := make(chan awake.Event)
	close(out)
	return out
}

func New(port Port, clock system.Clock, o Options) *Machine {
	if port == nil {
		port = absent{}
	}
	return &Machine{port: port, clock: clock, o: o}
}

func (m *Machine) caps() machine.Capabilities {
	return m.port.Capabilities().Onto(machine.Capabilities{})
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

func unsupported(err error) bool { return errors.Is(err, contract.ErrUnsupported) }

// invariant: resetting the clamshell is the safe direction, so a reset that
// fails is retried at every poll until it passes; a Mac with no power
// management has nothing to reset.
func (m *Machine) resetLocked() {
	err := m.port.Clamshell(false)
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

func (m *Machine) Awake(req Request) (held Session, refreshed bool, err error) {
	if req.For == 0 {
		req.For = Default
	}
	if bad := CheckFor(req.For); bad != nil {
		return Session{}, false, bad
	}
	r, err := m.port.Read()
	if unsupported(err) {
		return Session{}, false, &UnsupportedError{}
	}
	if err != nil {
		return Session{}, false, err
	}
	want := Session{Lid: req.Lid, Battery: req.Battery, By: req.By}
	if reason, _, words := trip(m.caps(), r, &want, 0); reason != "" {
		return Session{}, false, &GuardError{Guard: reason, Words: words}
	}
	m.mu.Lock()
	defer m.changed()
	defer m.mu.Unlock()
	m.reading = r
	now := m.clock.Now().UTC().Truncate(time.Second)
	want.StartedAt, want.Until = now, now.Add(req.For)
	refreshed = m.session != nil
	if refreshed {
		was := m.session.Lid
		want.StartedAt = m.session.StartedAt
		m.session = &want
		if want.Lid != was {
			if err := m.port.Clamshell(want.Lid); err != nil {
				m.log("clamshell: " + err.Error())
			}
		}
	} else {
		release, err := m.port.Hold("station: " + req.By)
		if err != nil {
			if unsupported(err) {
				return Session{}, false, &UnsupportedError{}
			}
			return Session{}, false, err
		}
		m.release, m.session = release, &want
		m.heavySince, m.polled = time.Time{}, now
		if want.Lid {
			if err := m.port.Clamshell(true); err != nil {
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

func (m *Machine) Stop() bool { return m.finish(ReasonStop, "station engine stop") }

func (m *Machine) Quit() {
	if !m.finish(ReasonQuit, "the engine stopped") {
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
	if r.Thermal == awake.ThermalHeavy && r.LidClosedOnBattery() {
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
	reason, detail, _ = trip(m.caps(), r, m.session, heavyFor)
	return reason, detail
}

func (m *Machine) Observe(ctx context.Context, ev awake.Event) {
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
		if err := m.port.Clamshell(true); err != nil {
			m.log("clamshell: " + err.Error())
		}
	}
	m.mu.Unlock()
	if lidHeld && ev.Kind == "lid" && r.Lid == "closed" {
		m.sleepDisplay(ctx)
	}
	m.changed()
}

func (m *Machine) sleepDisplay(ctx context.Context) {
	if err := m.port.DisplaySleepNow(ctx); err != nil {
		m.log("display: " + err.Error())
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
