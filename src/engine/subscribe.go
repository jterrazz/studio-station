package engine

import (
	"context"
	"encoding/json"
	"slices"
	"sync"
	"time"

	"github.com/jterrazz/studio-station/src/domain/power"
	"github.com/jterrazz/studio-station/src/system"
)

const (
	PagePower    = "power"
	PageMenubar  = "menubar"
	coalesce     = 250 * time.Millisecond
	tickEvery    = 15 * time.Second
	stallLimit   = 5 * time.Second
	streamSchema = "station.page/v1"
)

type FeedInput struct {
	// value: the daemon's own state moved (VMs, guests, leases, Power).
	Changed <-chan struct{}
	Refresh <-chan struct{}
	Power   func() power.View
}

type Feed func(ctx context.Context, in FeedInput) <-chan any

type subscriber struct {
	change  chan struct{}
	state   chan struct{}
	refresh chan struct{}
	notice  chan Notice
}

type hub struct {
	mu   sync.Mutex
	subs map[*subscriber]bool
	seen string
}

func (h *hub) add() *subscriber {
	s := &subscriber{change: make(chan struct{}, 1), state: make(chan struct{}, 1), refresh: make(chan struct{}, 1), notice: make(chan Notice, 8)}
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

func signal(c chan struct{}) {
	select {
	case c <- struct{}{}:
	default:
	}
}

// invariant: a subscriber that does not read misses signals, never blocks the
// daemon: every send is a non-blocking one into a small buffer.
func (h *hub) changed() {
	h.mu.Lock()
	defer h.mu.Unlock()
	for s := range h.subs {
		signal(s.change)
		signal(s.state)
	}
}

func (h *hub) refreshed() {
	h.mu.Lock()
	defer h.mu.Unlock()
	for s := range h.subs {
		signal(s.refresh)
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

func (h *hub) listening() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.subs) > 0
}

// invariant: the daemon's own state is compared each tick while a client
// subscribes, so a VM, guest or lease change reaches the stream within a
// tick; with no subscriber nothing is read.
func (d *Daemon) watchState() {
	if !d.hub.listening() {
		return
	}
	now := string(mustJSON([]any{d.vmViews(), d.hostView("")}))
	d.hub.mu.Lock()
	moved := d.hub.seen != now
	d.hub.seen = now
	d.hub.mu.Unlock()
	if moved {
		d.hub.changed()
	}
}

func (d *Daemon) known(id string) bool {
	if id == PagePower {
		return d.o.PowerPage != nil
	}
	_, ok := d.o.Feeds[id]
	return ok
}

func (d *Daemon) checkPages(pages []string) error {
	hint := "speak " + Schema + ": subscribe menubar power"
	if len(pages) == 0 {
		return fault("unknown_page", "subscribe names no page", hint)
	}
	for _, id := range pages {
		if !d.known(id) {
			return fault("unknown_page", "unknown page \""+id+"\"", hint)
		}
	}
	return nil
}

type latest struct {
	mu    sync.Mutex
	pages map[string]json.RawMessage
}

func (l *latest) set(id string, p json.RawMessage) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.pages[id] = p
}

func (l *latest) get(id string) (json.RawMessage, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	p, ok := l.pages[id]
	return p, ok
}

// invariant: a feed's first page is its frame and goes out before any
// other, never coalesced away by the reads that answer right after it.
func (d *Daemon) feed(ctx context.Context, id string, sub *subscriber, into *latest, first chan<- json.RawMessage) {
	in := FeedInput{Changed: sub.state, Refresh: sub.refresh, Power: d.power.View}
	framed := false
	for p := range d.o.Feeds[id](ctx, in) {
		data, err := json.Marshal(p)
		if err != nil {
			continue
		}
		if !framed {
			framed = true
			first <- data
			continue
		}
		into.set(id, data)
		signal(sub.change)
	}
	if !framed {
		close(first)
	}
}

// invariant: a write that does not finish within stallLimit closes the
// connection: the daemon never blocks on a subscriber that does not read.
func (d *Daemon) guarded(ctx context.Context, conn system.Conn, write func(any) error) func(any) error {
	return func(v any) error {
		wctx, done := context.WithCancel(ctx)
		defer done()
		go func() {
			if d.h.Clock.Sleep(wctx, d.stall()) == nil && wctx.Err() == nil {
				_ = conn.Close()
			}
		}()
		return write(v)
	}
}

func (d *Daemon) stall() time.Duration {
	if d.o.Stall > 0 {
		return d.o.Stall
	}
	return stallLimit
}

// invariant: after its ok, a subscription only streams: a full page per
// change, coalesced over 250 ms (so at most 4 a second) and never resent
// unchanged, a tick every 15 s, and the notices; a later request line on it
// is read and ignored.
func (d *Daemon) stream(ctx context.Context, conn system.Conn, r Request, write func(any) error) {
	sub := d.hub.add()
	defer d.hub.remove(sub)
	pages := slices.Compact(slices.Sorted(slices.Values(r.Pages)))
	if write(Response{V: 1, ID: r.ID, OK: true, Result: mustJSON(map[string]any{"stream": streamSchema, "pages": pages})}) != nil {
		return
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	write = d.guarded(ctx, conn, write)
	go func() {
		buf := make([]byte, 512)
		for {
			if _, err := conn.Read(buf); err != nil {
				cancel()
				return
			}
		}
	}()
	fed := &latest{pages: map[string]json.RawMessage{}}
	frames := map[string]chan json.RawMessage{}
	var feeds sync.WaitGroup
	for _, id := range pages {
		if id != PagePower {
			frames[id] = make(chan json.RawMessage, 1)
			feeds.Add(1)
			go func() {
				defer feeds.Done()
				d.feed(ctx, id, sub, fed, frames[id])
			}()
		}
	}
	defer feeds.Wait()
	defer cancel()

	ticks := make(chan struct{}, 1)
	go func() {
		for d.h.Clock.Sleep(ctx, tickEvery) == nil {
			signal(ticks)
		}
	}()
	last := map[string]string{}
	pageOf := func(id string) (json.RawMessage, bool) {
		if id == PagePower {
			data, err := json.Marshal(d.o.PowerPage(d.power.View()))
			return data, err == nil
		}
		return fed.get(id)
	}
	send := func(framed bool) bool {
		for _, id := range pages {
			if _, fed := frames[id]; fed && !framed {
				continue
			}
			p, ok := pageOf(id)
			if !ok || last[id] == string(p) {
				continue
			}
			last[id] = string(p)
			if write(Event{V: 1, Event: "page", Page: p}) != nil {
				return false
			}
		}
		return true
	}
	for _, id := range pages {
		frame, ok := frames[id]
		if !ok {
			continue
		}
		select {
		case p, ok := <-frame:
			if !ok {
				continue
			}
			last[id] = string(p)
			if write(Event{V: 1, Event: "page", Page: p}) != nil {
				return
			}
		case <-ctx.Done():
			return
		}
	}
	if !send(false) {
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
			if !send(true) {
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
