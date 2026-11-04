package engine

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jterrazz/studio-station/src/domain/power"
)

const NoticeEnded = "power_ended"

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
		Notice: func(e power.End) { d.hub.tell(Notice{Kind: NoticeEnded, Text: "Power ended: " + e.Detail}) },
		Change: d.hub.changed,
	})
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
		return s, fault("power_range", rng.Error(), "station power awake --for 4h")
	case errors.As(err, &guard):
		hint := "station power"
		if guard.Guard == power.ReasonLid {
			hint = "station power awake --lid --battery"
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
