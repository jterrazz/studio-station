package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"sync"
	"time"

	"github.com/jterrazz/studio-station/src/domain/power"
	"github.com/jterrazz/studio-station/src/system"
)

const (
	PagePower    = "power"
	coalesce     = 250 * time.Millisecond
	tickEvery    = 15 * time.Second
	NoticeEnded  = "power_ended"
	streamSchema = "station.page/v1"
)

type PowerSlept struct {
	Ended bool `json:"ended"`
	After int  `json:"after"`
}

type StopAll struct {
	VMs        []Stopped `json:"vms"`
	PowerEnded bool      `json:"power_ended"`
}

type Notice struct {
	Kind string `json:"kind"`
	Text string `json:"text"`
}

type Event struct {
	V      int             `json:"v"`
	Event  string          `json:"event"`
	Page   json.RawMessage `json:"page,omitempty"`
	At     string          `json:"at,omitempty"`
	Notice *Notice         `json:"notice,omitempty"`
}

type subscriber struct {
	change chan struct{}
	notice chan Notice
}

type hub struct {
	mu   sync.Mutex
	subs map[*subscriber]bool
}

func (h *hub) add() *subscriber {
	s := &subscriber{change: make(chan struct{}, 1), notice: make(chan Notice, 8)}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.subs == nil {
		h.subs = map[*subscriber]bool{}
	}
	h.subs[s] = true
	return s
}

func (h *hub) remove(s *subscriber) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.subs, s)
}

// invariant: a subscriber that does not read misses signals, never blocks the
// daemon: every send is a non-blocking one into a small buffer.
func (h *hub) changed() {
	h.mu.Lock()
	defer h.mu.Unlock()
	for s := range h.subs {
		select {
		case s.change <- struct{}{}:
		default:
		}
	}
}

func (h *hub) tell(n Notice) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for s := range h.subs {
		select {
		case s.notice <- n:
		default:
		}
	}
}

func (d *Daemon) newPower() *power.Machine {
	return power.New(d.h.Power, d.h.Run, d.h.Clock, power.Options{
		Pmset:  d.o.Pmset,
		Log:    func(m string) { d.logf("power", m) },
		Notice: func(e power.End) { d.hub.tell(Notice{Kind: NoticeEnded, Text: "Power ended: " + e.Detail}) },
		Change: d.hub.changed,
	})
}

func (d *Daemon) watchPower(ctx context.Context) {
	if d.h.Power == nil {
		return
	}
	for ev := range d.h.Power.Watch(ctx) {
		d.power.Observe(ctx, ev)
	}
}

func (d *Daemon) powerAwake(r Request) (power.Awoken, error) {
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

func (d *Daemon) pageOf(id string) (json.RawMessage, error) {
	if id != PagePower || d.o.PowerPage == nil {
		return nil, fault("unknown_page", "unknown page \""+id+"\"", "speak "+Schema+": subscribe power")
	}
	return json.Marshal(d.o.PowerPage(d.power.View()))
}

func (d *Daemon) checkPages(pages []string) error {
	if len(pages) == 0 {
		return fault("unknown_page", "subscribe names no page", "speak "+Schema+": subscribe power")
	}
	for _, id := range pages {
		if _, err := d.pageOf(id); err != nil {
			return err
		}
	}
	return nil
}

// invariant: after its ok, a subscription only streams: a full page per
// change, coalesced over 250 ms and never resent unchanged, a tick every
// 15 s, and the notices; a later request line on it is read and ignored.
func (d *Daemon) stream(ctx context.Context, conn system.Conn, r Request, write func(any) error) {
	sub := d.hub.add()
	defer d.hub.remove(sub)
	pages := slices.Compact(slices.Sorted(slices.Values(r.Pages)))
	if write(Response{V: 1, ID: r.ID, OK: true, Result: mustJSON(map[string]any{"stream": streamSchema, "pages": pages})}) != nil {
		return
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		buf := make([]byte, 512)
		for {
			if _, err := conn.Read(buf); err != nil {
				cancel()
				return
			}
		}
	}()
	ticks := make(chan struct{}, 1)
	go func() {
		for d.h.Clock.Sleep(ctx, tickEvery) == nil {
			select {
			case ticks <- struct{}{}:
			default:
			}
		}
	}()
	last := map[string]string{}
	send := func() bool {
		for _, id := range pages {
			p, err := d.pageOf(id)
			if err != nil || last[id] == string(p) {
				continue
			}
			last[id] = string(p)
			if write(Event{V: 1, Event: "page", Page: p}) != nil {
				return false
			}
		}
		return true
	}
	if !send() {
		return
	}
	for {
		select {
		case <-ctx.Done():
			return
		case <-sub.change:
			if d.h.Clock.Sleep(ctx, coalesce) != nil {
				return
			}
			select {
			case <-sub.change:
			default:
			}
			if !send() {
				return
			}
		case n := <-sub.notice:
			if write(Event{V: 1, Event: "notice", Notice: &n}) != nil {
				return
			}
		case <-ticks:
			if write(Event{V: 1, Event: "tick", At: d.h.Clock.Now().UTC().Format(time.RFC3339)}) != nil {
				return
			}
		}
	}
}

func mustJSON(v any) json.RawMessage {
	data, _ := json.Marshal(v)
	return data
}
