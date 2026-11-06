package process

import (
	"context"
	"errors"
	"time"

	"github.com/jterrazz/studio-station/src/awake"
	"github.com/jterrazz/studio-station/src/catalog"

	awakecalls "github.com/jterrazz/studio-station/src/awake/calls"
	"github.com/jterrazz/studio-station/src/engine"
)

func (d *Daemon) newPower() *awakecalls.Keeper {
	return awakecalls.NewKeeper(d.o.Power, d.h, awakecalls.KeeperOptions{
		Log:    func(m string) { d.logf("power", m) },
		Notice: func(e awake.End) { d.told(engine.NoticeEnded, engine.AwakeStopped(e.Detail)) },
		Told:   func(text string) { d.told(engine.NoticeNeverSleeps, text) },
		Change: d.hub.changed,
	})
}

// invariant: every end of Awake and every word of Never sleeps is told once,
// to the open surfaces and to the owner's notifications.
func (d *Daemon) told(kind, text string) {
	d.hub.tell(engine.Notice{Kind: kind, Text: text})
	d.notify(context.Background(), engine.Notification{Kind: kind, Subject: "awake", Text: text})
}

// invariant: at its start the engine holds Never sleeps before any VM
// starts, as held since that start.
func (d *Daemon) neverSleeps(since time.Time) {
	if d.o.NeverSleeps != nil {
		d.power.Policy(d.o.NeverSleeps(), since)
	}
}

func (d *Daemon) AwakeKept() awake.Kept {
	// value: what this engine keeps of Awake, for the stepped engine that
	// follows it.
	return d.power.Kept()
}

func (d *Daemon) RememberAwake(k awake.Kept) { d.awake = &k }

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

func (d *Daemon) powerAwake(r engine.AwakeIn) (awake.Awoken, error) {
	session, refreshed, err := d.power.Awake(awake.Request{Lid: r.Lid, Battery: r.Battery, For: time.Duration(r.For) * time.Second, By: r.By})
	s := awake.Awoken{Session: session, Refreshed: refreshed}
	var rng *awake.RangeError
	var guard *awake.GuardError
	var none *awake.UnsupportedError
	switch {
	case errors.As(err, &rng):
		return s, fault("power_range", rng.Error(), "station awake 4h")
	case errors.As(err, &guard):
		hint := "station awake"
		if guard.Guard == awake.ReasonLid {
			hint = "station awake 4h --lid --battery"
		}
		return s, fault("power_guard", guard.Error(), hint)
	case errors.As(err, &none):
		return s, fault("power_unsupported", none.Error(), catalog.PowerOnItsMac)
	}
	return s, err
}

func (d *Daemon) powerSleep() engine.PowerSlept {
	s := d.power.Sleep()
	return engine.PowerSlept{Ended: s.Ended, After: int(s.After.Seconds())}
}
