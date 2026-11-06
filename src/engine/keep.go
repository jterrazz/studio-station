package engine

import (
	"context"
	"net/netip"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/jterrazz/studio-station/src/domain/machines"
	model "github.com/jterrazz/studio-station/src/machines"
)

type CheckRunner interface {
	// invariant: what the engine needs to run a server's check from its
	// Mac: nil once it answered within its budget, with what it answered.
	Run(ctx context.Context, t model.CheckTarget) (string, error)
}

// value: the closed list of what the engine tells the owner of its
// servers (SPEC §10.4), each a Notification's Kind.
const (
	EventRestarting = "server_restarting"
	EventLoop       = "server_loop"
	EventPoweredOff = "server_powered_off"
	EventBack       = "server_back"
)

const EventsStream = "engine.events" // value: the name the events watch opens with, as a page watch with its schema.

func Notifies(kind string) bool {
	// value: the closed list of SPEC §10.4: the events the engine's events
	// watch streams, which the menu bar posts as its notifications.
	return slices.Contains([]string{EventRestarting, EventLoop, EventPoweredOff, EventBack, NoticeEnded, EventExposed, EventDiskLow}, kind)
}

type Told struct {
	// value: an event of SPEC 10.4: its name, and the parts its text joins.
	Kind  string
	Parts []string
}

func Restarting(vm string, count int) Told {
	return Told{EventRestarting, []string{vm + " stopped and is restarting (↺ " + strconv.Itoa(count) + ")"}}
}

func Looping(vm string, r model.Restarts) Told {
	return Told{EventLoop, []string{vm + " is in a loop: " + strconv.Itoa(len(r.Recent)) + " restarts in " + strconv.Itoa(int(model.LoopWindow.Minutes())) + " min", "waiting " + strconv.Itoa(int(r.Wait.Seconds())) + " s"}}
}

func PoweredOff(vm string) Told {
	return Told{EventPoweredOff, []string{vm + " " + model.PoweredOffInside, "it stays stopped"}}
}

func Back(vm string) Told {
	return Told{EventBack, []string{vm + " is back", "all 4 health steps pass"}}
}

// invariant: what the engine tells goes to the Notifier and to every open
// watch alike, its parts joined as the presentation joins them.
func (d *Daemon) tell(ctx context.Context, vm string, t Told) {
	text := strings.Join(t.Parts, ", ")
	if d.o.Join != nil {
		text = d.o.Join(t.Parts...)
	}
	d.notify(ctx, Notification{Kind: t.Kind, Subject: vm, Text: text})
	d.hub.tell(Notice{Kind: t.Kind, Text: text})
}

// invariant: a server whose run ended unexpectedly boots again after its
// wait (SPEC §7.4); the owner is told of each restart, and once of a loop.
func (d *Daemon) restartLater(ctx context.Context, v *vmState, up time.Duration, now time.Time) {
	d.mu.Lock()
	was := v.restarts.Looping
	v.restarts = v.restarts.After(up, now)
	r := v.restarts
	v.rerunAt = now.Add(r.Wait)
	v.state, v.detail = StateStopped, "restarting"
	d.mu.Unlock()
	switch {
	case r.Looping && !was:
		d.tell(ctx, v.rec.Name, Looping(v.rec.Name, r))
	case !r.Looping:
		d.tell(ctx, v.rec.Name, Restarting(v.rec.Name, r.Count))
	}
}

// invariant: a running server's health check runs after its boot once it
// has an address, then every minute, never two at once; a loop ends once a
// run stays up 10 min. A failing check never restarts it.
func (d *Daemon) keep(ctx context.Context, v *vmState) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if v.rec.Lifetime != machines.AlwaysOn || v.run == nil || v.stopping || v.state != StateRunning {
		return
	}
	now := d.h.Clock.Now()
	v.restarts = v.restarts.Steady(now.Sub(v.bootAt), now)
	check, ok := model.ParseCheck(v.rec.Check)
	addr, err := netip.ParseAddr(v.ip)
	if d.o.Checks == nil || !ok || err != nil || v.checking || !v.check.Due(now) {
		return
	}
	v.checking = true
	target := check.Target(v.rec.Name, addr)
	if d.life != nil {
		ctx = d.life
	}
	go func() {
		answer, err := d.o.Checks.Run(ctx, target)
		passed := err == nil
		d.mu.Lock()
		var change model.CheckChange
		v.check, change = model.Health(v.check, passed, now)
		if passed {
			v.check.Answer = answer
		}
		v.checking = false
		d.mu.Unlock()
		switch change {
		case model.CheckFails:
			d.logf(v.rec.Name, "its check fails: "+v.rec.Check)
		case model.CheckBack:
			d.tell(ctx, v.rec.Name, Back(v.rec.Name))
		case model.CheckSame:
		}
	}()
}

func (d *Daemon) Events(ctx context.Context, send func(any) error) error {
	// invariant: the events watch streams each event of the closed list the
	// engine tells, once, as it is told, until the watcher leaves (the menu
	// bar's notifications); any other word it says is not one.
	sub := d.hub.add()
	defer d.hub.remove(sub)
	if send(Opened{Stream: EventsStream}) != nil {
		return nil
	}
	for {
		select {
		case <-ctx.Done():
			return nil
		case n := <-sub.notice:
			if !Notifies(n.Kind) {
				continue
			}
			if err := send(Event{Event: "notice", Notice: &n}); err != nil {
				return nil
			}
		}
	}
}
