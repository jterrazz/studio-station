package engine

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jterrazz/studio-station/src/domain/power"
)

const (
	NoticeEnded       = "power_ended"
	NoticeNeverSleeps = "never_sleeps"
)

type PowerSlept struct {
	Ended bool `json:"ended"`
	After int  `json:"after"`
}

type StopAll struct {
	VMs        []Stopped `json:"vms"`
	PowerEnded bool      `json:"power_ended"`
	// value: a port of a VM it stopped was shared, and is no more.
	SharingEnded bool `json:"sharing_ended,omitempty"`
}

type Notice struct {
	Kind string `json:"kind"`
	Text string `json:"text"`
}

type Event struct {
	Event  string          `json:"event"`
	Page   json.RawMessage `json:"page,omitempty"`
	At     string          `json:"at,omitempty"`
	Notice *Notice         `json:"notice,omitempty"`
}

func (d *Daemon) newPower() *power.Machine {
	return power.New(d.o.Power, d.h.Clock, power.Options{
		Log:    func(m string) { d.logf("power", m) },
		Notice: func(e power.End) { d.told(NoticeEnded, "Awake stopped: "+e.Detail) },
		Told:   func(text string) { d.told(NoticeNeverSleeps, text) },
		Change: d.hub.changed,
	})
}

// invariant: every end of Awake and every word of Never sleeps is told once,
// to the open surfaces and to the owner's notifications.
func (d *Daemon) told(kind, text string) {
	d.hub.tell(Notice{Kind: kind, Text: text})
	d.notify(context.Background(), Notification{Kind: kind, Subject: "awake", Text: text})
}

// invariant: at its start the engine holds Never sleeps before any VM
// starts, as held since that start.
func (d *Daemon) neverSleeps(since time.Time) {
	if d.o.NeverSleeps != nil {
		d.power.Policy(d.o.NeverSleeps(), since)
	}
}

type AwakeKept = power.Kept

func (d *Daemon) AwakeKept() power.Kept {
	// value: what this engine keeps of Awake, for the stepped engine that
	// follows it.
	return d.power.Kept()
}

func (d *Daemon) RememberAwake(k power.Kept) { d.awake = &k }

func (d *Daemon) Suspend() {
	d.power.Suspend()
	d.closeAll()
}

func (d *Daemon) watchPower(ctx context.Context) {
	if d.o.Power == nil {
		return
	}
	for ev := range d.o.Power.Watch(ctx) {
		d.power.Observe(ctx, ev)
	}
}

func (d *Daemon) powerAwake(r AwakeIn) (power.Awoken, error) {
	session, refreshed, err := d.power.Awake(power.Request{Lid: r.Lid, Battery: r.Battery, For: time.Duration(r.For) * time.Second, By: r.By})
	s := power.Awoken{Session: session, Refreshed: refreshed}
	var rng *power.RangeError
	var guard *power.GuardError
	var none *power.UnsupportedError
	switch {
	case errors.As(err, &rng):
		return s, fault("power_range", rng.Error(), "station awake 4h")
	case errors.As(err, &guard):
		hint := "station awake"
		if guard.Guard == power.ReasonLid {
			hint = "station awake 4h --lid --battery"
		}
		return s, fault("power_guard", guard.Error(), hint)
	case errors.As(err, &none):
		return s, fault("power_unsupported", none.Error(), "ssh <host> station power")
	}
	return s, err
}

func (d *Daemon) powerSleep() PowerSlept {
	s := d.power.Sleep()
	return PowerSlept{Ended: s.Ended, After: int(s.After.Seconds())}
}
