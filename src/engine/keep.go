package engine

import (
	"context"
	"net/netip"
	"strconv"
	"strings"
	"time"

	"github.com/jterrazz/studio-station/src/domain/machines"
	model "github.com/jterrazz/studio-station/src/machines"
)

type HealthProbe interface {
	// invariant: what the engine needs to run a server's check from its
	// Mac: nil once it answered within its budget.
	Probe(ctx context.Context, t model.CheckTarget) error
}

// value: the closed list of what the engine tells the owner of its
// servers (SPEC §10.4), each a Notification's Kind.
const (
	EventRestarting = "server_restarting"
	EventLoop       = "server_loop"
	EventPoweredOff = "server_powered_off"
	EventBack       = "server_back"
)

// value: each event's words, in the parts its text joins (SPEC §10.4).
func Restarting(vm string, count int) (string, []string) {
	return EventRestarting, []string{vm + " stopped and is restarting (↺ " + strconv.Itoa(count) + ")"}
}

func Looping(vm string, r model.Restarts) (string, []string) {
	return EventLoop, []string{vm + " is in a loop: " + strconv.Itoa(len(r.Recent)) + " restarts in " + strconv.Itoa(int(model.LoopWindow.Minutes())) + " min", "waiting " + strconv.Itoa(int(r.Wait.Seconds())) + " s"}
}

func PoweredOff(vm string) (string, []string) {
	return EventPoweredOff, []string{vm + " " + model.PoweredOffInside, "it stays stopped"}
}

func Back(vm string) (string, []string) {
	return EventBack, []string{vm + " is back", "all 4 health steps pass"}
}

// invariant: what the engine tells goes to the Notifier and to every open
// watch alike, its parts joined as the presentation joins them.
func (d *Daemon) tell(ctx context.Context, vm, kind string, parts []string) {
	text := strings.Join(parts, ", ")
	if d.o.Join != nil {
		text = d.o.Join(parts...)
	}
	d.notify(ctx, Notification{Kind: kind, Subject: vm, Text: text})
	d.hub.tell(Notice{Kind: kind, Text: text})
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
		kind, parts := Looping(v.rec.Name, r)
		d.tell(ctx, v.rec.Name, kind, parts)
	case !r.Looping:
		kind, parts := Restarting(v.rec.Name, r.Count)
		d.tell(ctx, v.rec.Name, kind, parts)
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
		passed := d.o.Checks.Probe(ctx, target) == nil
		d.mu.Lock()
		var change model.CheckChange
		v.check, change = model.Health(v.check, passed, now)
		v.checking = false
		d.mu.Unlock()
		switch change {
		case model.CheckFails:
			d.logf(v.rec.Name, "its check fails: "+v.rec.Check)
		case model.CheckBack:
			kind, parts := Back(v.rec.Name)
			d.tell(ctx, v.rec.Name, kind, parts)
		case model.CheckSame:
		}
	}()
}

// invariant: the events watch streams what the engine tells, each once, as
// it is told, until the watcher leaves (the menu bar's notifications).
func (d *Daemon) Events(ctx context.Context, send func(any) error) error {
	sub := d.hub.add()
	defer d.hub.remove(sub)
	for {
		select {
		case <-ctx.Done():
			return nil
		case n := <-sub.notice:
			if err := send(Event{Event: "notice", Notice: &n}); err != nil {
				return nil
			}
		}
	}
}
