package engine

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/domain/settings"
	"github.com/jterrazz/studio-station/src/system/systemtest"
)

type sleeper struct {
	at      time.Time
	asked   time.Duration
	release chan struct{}
}

type manualClock struct {
	mu       sync.Mutex
	now      time.Time
	sleepers map[*sleeper]bool
}

func newManualClock() *manualClock {
	return &manualClock{now: time.Date(2026, 11, 3, 10, 30, 0, 0, time.UTC), sleepers: map[*sleeper]bool{}}
}

func (c *manualClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *manualClock) Sleep(ctx context.Context, d time.Duration) error {
	c.mu.Lock()
	s := &sleeper{at: c.now.Add(d), asked: d, release: make(chan struct{})}
	c.sleepers[s] = true
	c.mu.Unlock()
	select {
	case <-s.release:
		return nil
	case <-ctx.Done():
		c.mu.Lock()
		delete(c.sleepers, s)
		c.mu.Unlock()
		return ctx.Err()
	}
}

func (c *manualClock) await(d time.Duration) {
	for {
		c.mu.Lock()
		for s := range c.sleepers {
			if s.asked == d {
				c.mu.Unlock()
				return
			}
		}
		c.mu.Unlock()
		runtime.Gosched()
	}
}

// invariant: a write that just finished may not have cancelled its stall
// watch yet; a jump of the clock waits for it, unless it is the stall's own.
func (c *manualClock) settled(d time.Duration) {
	for d != stallLimit {
		c.mu.Lock()
		busy := false
		for s := range c.sleepers {
			busy = busy || s.asked == stallLimit
		}
		c.mu.Unlock()
		if !busy {
			return
		}
		runtime.Gosched()
	}
}

func (c *manualClock) advance(d time.Duration) {
	c.await(d)
	c.settled(d)
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
	for s := range c.sleepers {
		if !s.at.After(c.now) {
			delete(c.sleepers, s)
			close(s.release)
		}
	}
}

type pipeConn struct {
	io.Reader
	io.Writer
	closers []io.Closer
}

func (p pipeConn) Close() error {
	for _, c := range p.closers {
		_ = c.Close()
	}
	return nil
}

type streamed struct {
	events chan Event
	ended  chan struct{}
}

// value: the nth subscription's feed reads the nth channel.
func streamDaemon(t *testing.T, clock *manualClock, feeds ...chan any) *Daemon {
	t.Helper()
	files := systemtest.NewFS()
	h := systemtest.Host(systemtest.NewRunner(t), files, systemtest.Env{}, systemtest.NewNet(), nil)
	h.Clock = clock
	paths := settings.Paths{Config: "/c", State: "/s"}
	var n atomic.Int32
	return New(h, newHypervisor(), nil, &fakeMachines{}, &fakeTailnet{}, memStores(), Options{State: paths.State, Here: catalog.Host,
		Feeds: map[string]Feed{PageMenubar: func(ctx context.Context, _ FeedInput) <-chan any {
			feed := feeds[n.Add(1)-1]
			out := make(chan any)
			go func() {
				defer close(out)
				for {
					select {
					case <-ctx.Done():
						return
					case p := <-feed:
						select {
						case out <- p:
						case <-ctx.Done():
							return
						}
					}
				}
			}()
			return out
		}}})
}

func subscribe(ctx context.Context, t *testing.T, d *Daemon, reading bool) streamed {
	t.Helper()
	inR, inW := io.Pipe()
	outR, outW := io.Pipe()
	server := pipeConn{Reader: inR, Writer: outW, closers: []io.Closer{inR, outW}}
	s := streamed{events: make(chan Event, 64), ended: make(chan struct{})}
	go func() {
		d.serveConn(ctx, server)
		close(s.ended)
	}()
	go func() { _, _ = inW.Write([]byte(`{"v":1,"id":"1","op":"subscribe","pages":["menubar"]}` + "\n")) }()
	r := bufio.NewReader(outR)
	if _, err := r.ReadBytes('\n'); err != nil {
		t.Fatal(err)
	}
	if reading {
		go func() {
			defer close(s.events)
			for {
				line, err := r.ReadBytes('\n')
				if err != nil {
					return
				}
				var ev Event
				_ = json.Unmarshal(line, &ev)
				s.events <- ev
			}
		}()
	}
	return s
}

func (s streamed) next(t *testing.T, kind string) string {
	t.Helper()
	for ev := range s.events {
		if ev.Event == kind {
			if kind == "page" {
				return string(ev.Page)
			}
			return ev.At
		}
	}
	t.Fatalf("the stream ended before a %s", kind)
	return ""
}

// value: the third send returns only once the feed's loop is back for more,
// so the first one is stored.
func settle(feed chan any, v any) {
	for range 3 {
		feed <- v
	}
}

func TestAStreamSendsTheFrameThenOnePageForABurstAndNeverTheSamePageTwice(t *testing.T) {
	clock, feed := newManualClock(), make(chan any)
	s := subscribe(t.Context(), t, streamDaemon(t, clock, feed), true)
	feed <- "frame"
	if got := s.next(t, "page"); got != `"frame"` {
		t.Fatalf("first page %s, want the frame", got)
	}
	feed <- "a"
	feed <- "b"
	settle(feed, "c")
	clock.advance(coalesce)
	if got := s.next(t, "page"); got != `"c"` {
		t.Fatalf("after a burst %s, want only its last page", got)
	}
	settle(feed, "c")
	settle(feed, "d")
	clock.advance(coalesce)
	if got := s.next(t, "page"); got != `"d"` {
		t.Fatalf("next page %s, want d: an equal page is never resent", got)
	}
}

func TestAStreamTicksEvery15Seconds(t *testing.T) {
	clock, feed := newManualClock(), make(chan any)
	s := subscribe(t.Context(), t, streamDaemon(t, clock, feed), true)
	feed <- "frame"
	s.next(t, "page")
	clock.advance(tickEvery)
	if got := s.next(t, "tick"); got != "2026-11-03T10:30:15Z" {
		t.Fatalf("tick at %s, want 15 s after the subscription", got)
	}
}

func TestASubscriberThatDoesNotReadIsDroppedWhileAnotherKeepsItsPages(t *testing.T) {
	clock, feedA, feedB := newManualClock(), make(chan any), make(chan any)
	d := streamDaemon(t, clock, feedA, feedB)
	reader := subscribe(t.Context(), t, d, true)
	feedA <- "frame"
	reader.next(t, "page")
	clock.settled(0)
	stalled := subscribe(t.Context(), t, d, false)
	feedB <- "frame"
	clock.advance(stallLimit)
	<-stalled.ended
	settle(feedA, "after")
	clock.advance(coalesce)
	if got := reader.next(t, "page"); got != `"after"` {
		t.Fatalf("the reader got %s, want its page after the other was dropped", got)
	}
}
