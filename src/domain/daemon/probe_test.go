package daemon

import (
	"context"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"

	"github.com/jterrazz/studio-station/src/domain/services"
	"github.com/jterrazz/studio-station/src/system"
)

type wallConn struct{ wrote chan []byte }

func (c wallConn) Read([]byte) (int, error) { return 0, io.EOF }

func (c wallConn) Write(p []byte) (int, error) {
	c.wrote <- append([]byte(nil), p...)
	return len(p), nil
}

func (c wallConn) Close() error { return nil }

type wallListener struct {
	conns chan wallConn
	once  sync.Once
	done  chan struct{}
}

func (l *wallListener) Accept() (system.Conn, error) {
	select {
	case c := <-l.conns:
		return c, nil
	case <-l.done:
		return nil, errors.New("closed")
	}
}

func (l *wallListener) Close() error {
	l.once.Do(func() { close(l.done) })
	return nil
}

func (l *wallListener) Port() int { return 40999 }

type wall struct {
	system.Sockets
	drops bool
	mu    sync.Mutex
	bound []string
	l     *wallListener
}

func (w *wall) ListenTCP(ip string, _ int) (system.Listener, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.bound = append(w.bound, ip)
	w.l = &wallListener{conns: make(chan wallConn, 1), done: make(chan struct{})}
	return w.l, nil
}

func (w *wall) Pipe(_ context.Context, _ string, _ int, _ io.Reader, out io.Writer) error {
	w.mu.Lock()
	l := w.l
	w.mu.Unlock()
	conn := wallConn{wrote: make(chan []byte, 1)}
	l.conns <- conn
	data := <-conn.wrote
	if w.drops {
		return nil
	}
	_, err := out.Write(data)
	return err
}

func TestTheTailnetProbeCountsBytesNotTheHandshake(t *testing.T) {
	for _, drops := range []bool{false, true} {
		h := newHarness(t)
		h.d.h.Sockets = &wall{drops: drops}
		if got := h.d.carries(t.Context(), "100.64.0.1"); got == drops {
			t.Errorf("a firewall that drops=%v: carries %v, want %v", drops, got, !drops)
		}
	}
}

func TestBehindTheFirewallAPublishBindsTheLoopbackOnlyAndSaysWhy(t *testing.T) {
	h := newHarness(t)
	w := &wall{drops: true}
	h.d.h.Sockets = w
	h.d.self.IP, h.d.walled = "100.64.0.1", true
	tcp := false
	h.d.leases = []services.Lease{{VM: "api", GuestPort: 9000, HostPort: 40000, HTTP: &tcp}}
	s := h.d.svcOf("api", 9000)
	h.d.publish(t.Context(), s)
	defer h.d.unpublish(t.Context(), s)
	if strings.Join(w.bound, " ") != "127.0.0.1" || s.detail != "firewall: local only" {
		t.Errorf("bound %v with detail %q, want 127.0.0.1 only and \"firewall: local only\"", w.bound, s.detail)
	}
}
