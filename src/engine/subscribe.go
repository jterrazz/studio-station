package engine

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/jterrazz/studio-station/src/domain/power"
)

const (
	PagePower   = "power"
	PageMenubar = "menubar"
	// value: the menu bar's pages of SPEC §10.2, each watched by its name
	// and, after a colon, its subject: `menubar.machine:infra`, that
	// machine's page; `menubar.ports:jterrazz-mac-mini`, the ports of that
	// Mac; `menubar.activity:6m`, This Mac over 6 min.
	PageSwitcher = "menubar.switcher"
	PageMachine  = "menubar.machine"
	PagePorts    = "menubar.ports"
	PageAwake    = "menubar.awake"
	PageActivity = "menubar.activity"
	coalesce     = 250 * time.Millisecond
	tickEvery    = 15 * time.Second
	streamSchema = "station.page/v1"
)

type FeedInput struct {
	// value: the daemon's own state moved (VMs, guests, leases, Power).
	Changed <-chan struct{}
	Refresh <-chan struct{}
	Power   func() power.View
	// value: what a page is of, after its page's name (`menubar.machine:infra`,
	// `menubar.ports:jterrazz-mac-mini`); empty for every other page.
	Subject string
}

// value: a watched page's feed and its subject, `<feed>:<subject>`.
func feedOf(id string) (feed, subject string) {
	feed, subject, _ = strings.Cut(id, ":")
	return feed, subject
}

type Feed func(ctx context.Context, in FeedInput) <-chan any

type subscriber struct {
	change chan struct{}
	notice chan Notice
	inlets map[string]inlet
}

// value: what wakes one watched page's feed: the engine's state moving, and
// a refresh that names that page.
type inlet struct {
	state   chan struct{}
	refresh chan struct{}
}

type hub struct {
	mu   sync.Mutex
	subs map[*subscriber]bool
	seen string
}

func (h *hub) add(pages ...string) *subscriber {
	s := &subscriber{change: make(chan struct{}, 1), notice: make(chan Notice, 8), inlets: map[string]inlet{}}
	for _, id := range pages {
		s.inlets[id] = inlet{state: make(chan struct{}, 1), refresh: make(chan struct{}, 1)}
	}
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
		for _, in := range s.inlets {
			signal(in.state)
		}
	}
}

func (h *hub) refreshed(pages []string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for s := range h.subs {
		for _, id := range pages {
			if in, ok := s.inlets[id]; ok {
				signal(in.refresh)
			}
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

func (h *hub) listening() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.subs) > 0
}

// invariant: the daemon's own state is compared each tick while a client
// subscribes, so a VM, guest or lease change reaches the stream within a
// tick; with no subscriber nothing is read.
func (d *Engine) watchState() {
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

func (d *Engine) known(id string) bool {
	if id == PagePower {
		return d.o.PowerPage != nil
	}
	_, ok := d.o.Feeds[id]
	if feed, subject := feedOf(id); !ok && subject != "" {
		_, ok = d.o.Feeds[feed]
	}
	return ok
}

func (d *Engine) CheckPages(pages []string) error {
	hint := "watch page with the pages menubar, menubar.switcher, menubar.machine:<name>, menubar.ports[:<mac>], menubar.awake, menubar.activity[:<window>] or power"
	if len(pages) == 0 {
		return fault("unknown_page", "the watch names no page", hint)
	}
	for _, id := range pages {
		if !d.known(id) {
			return fault("unknown_page", "unknown page \""+id+"\"", hint)
		}
	}
	return nil
}

type Opened struct {
	Stream string   `json:"stream"`
	Pages  []string `json:"pages,omitempty"`
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
func (d *Engine) feed(ctx context.Context, id string, sub *subscriber, into *latest, first chan<- json.RawMessage) {
	in := FeedInput{Changed: sub.inlets[id].state, Refresh: sub.inlets[id].refresh, Power: d.power.View}
	feed, ok := d.o.Feeds[id]
	if !ok {
		var name string
		name, in.Subject = feedOf(id)
		feed = d.o.Feeds[name]
	}
	framed := false
	for p := range feed(ctx, in) {
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

func (d *Engine) Stream(ctx context.Context, pages []string, write func(any) error) error {
	// invariant: after its opening, a watch of pages only streams: a full page
	// per change, coalesced over 250 ms (so at most 4 a second) and never resent
	// unchanged, a tick every 15 s, and the notices.
	if err := d.CheckPages(pages); err != nil {
		return err
	}
	pages = slices.Compact(slices.Sorted(slices.Values(pages)))
	sub := d.hub.add(pages...)
	defer d.hub.remove(sub)
	if err := write(Opened{Stream: streamSchema, Pages: pages}); err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
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
			if write(Event{Event: "page", Page: p}) != nil {
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
			if write(Event{Event: "page", Page: p}) != nil {
				return nil
			}
		case <-ctx.Done():
			return nil
		}
	}
	if !send(false) {
		return nil
	}
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-sub.change:
			if d.h.Clock.Sleep(ctx, coalesce) != nil {
				return nil
			}
			select {
			case <-sub.change:
			default:
			}
			if !send(true) {
				return nil
			}
		case n := <-sub.notice:
			if write(Event{Event: "notice", Notice: &n}) != nil {
				return nil
			}
		case <-ticks:
			if write(Event{Event: "tick", At: d.h.Clock.Now().UTC().Format(time.RFC3339)}) != nil {
				return nil
			}
		}
	}
}

func mustJSON(v any) json.RawMessage {
	data, _ := json.Marshal(v)
	return data
}
