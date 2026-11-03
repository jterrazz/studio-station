package engine

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/domain/machines"
	"github.com/jterrazz/studio-station/src/domain/services"
	"github.com/jterrazz/studio-station/src/domain/tailnet"
	"github.com/jterrazz/studio-station/src/system"
)

type fakeTailnet struct {
	mu      sync.Mutex
	self    tailnet.Self
	serve   tailnet.ServeConfig
	calls   []string
	certs   []string
	hold    chan struct{}
	certErr error
	until   time.Time
	entered chan string
}

func (f *fakeTailnet) Cert(ctx context.Context, host string) error {
	f.mu.Lock()
	f.certs = append(f.certs, host)
	hold, err := f.hold, f.certErr
	f.until, _ = ctx.Deadline()
	f.mu.Unlock()
	if f.entered != nil {
		f.entered <- host
	}
	if hold != nil {
		<-hold
	}
	return err
}

func (f *fakeTailnet) issued() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.certs...)
}

func (f *fakeTailnet) Self(context.Context) (tailnet.Self, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.self, nil
}

func (f *fakeTailnet) Serve(_ context.Context, args ...string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, strings.Join(args, " "))
	return nil
}

func (f *fakeTailnet) ServeStatus(context.Context) (tailnet.ServeConfig, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, "status")
	return f.serve, nil
}

func (f *fakeTailnet) turn(on bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.self.ServeOn = on
}

func (f *fakeTailnet) taken() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := f.calls
	f.calls = nil
	return out
}

type idleListener struct {
	once sync.Once
	done chan struct{}
}

func (l *idleListener) Accept() (system.Conn, error) {
	<-l.done
	return nil, errors.New("closed")
}

func (l *idleListener) Close() error {
	l.once.Do(func() { close(l.done) })
	return nil
}

func (l *idleListener) Port() int { return 0 }

type loopback struct {
	system.Sockets
	mu    sync.Mutex
	bound []string
}

func (s *loopback) ListenTCP(ip string, _ int) (system.Listener, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.bound = append(s.bound, ip)
	return &idleListener{done: make(chan struct{})}, nil
}

func TestServeIsReadEveryThirtyTicksAndOnlyServesWhileOn(t *testing.T) {
	h := newHarness(t)
	tn := &fakeTailnet{
		self:  tailnet.Self{DNSName: "x.tail1234.ts.net", BackendState: "Running"},
		serve: tailnet.ServeConfig{Entries: []tailnet.ServeEntry{{Port: 40099, TCP: true, Target: "127.0.0.1:40099"}}},
	}
	sockets := &loopback{}
	h.d.tn, h.d.h.Sockets = tn, sockets
	ready := certLines(h.d)
	ctx := t.Context()
	yes := true
	h.d.leases = []services.Lease{{VM: "api", GuestPort: 8000, HostPort: 40000, HTTP: &yes}}
	h.d.refreshSelf(ctx)
	h.d.mu.Lock()
	s := h.d.svcOf("api", 8000)
	s.st.Phase = services.Published
	h.d.publish(ctx, s)
	h.d.mu.Unlock()
	defer h.d.closeAll()
	row := func() services.View { return h.d.hostView("").Services[0] }
	if calls := tn.taken(); len(calls) != 0 || row().Detail != catalog.ServeOffDetail || row().URLs.Public != "http://127.0.0.1:40000" {
		t.Fatalf("Serve off: calls %q, row %+v; want no serve, the admin step and the local URL", calls, row())
	}
	tn.turn(true)
	for tick := 1; tick < serveEvery; tick++ {
		h.d.step(ctx, tick)
	}
	if calls := tn.taken(); len(calls) != 0 {
		t.Fatalf("Serve turned on was acted on before its read: %q", calls)
	}
	h.d.step(ctx, serveEvery)
	<-ready
	want := []string{"--bg --https=40000 http://127.0.0.1:40000", "status", "--tcp=40099 off"}
	if calls := tn.taken(); strings.Join(calls, "|") != strings.Join(want, "|") {
		t.Fatalf("Serve on at the next read: calls %q, want %q", calls, want)
	}
	if r := row(); r.Detail != "" || r.URLs.Public != "https://x.tail1234.ts.net:40000" {
		t.Fatalf("Serve on: row %+v, want a clean https publish", r)
	}
	tn.turn(false)
	for tick := serveEvery + 1; tick <= 2*serveEvery; tick++ {
		h.d.step(ctx, tick)
	}
	h.d.mu.Lock()
	h.d.unpublish(ctx, s)
	h.d.mu.Unlock()
	if calls := tn.taken(); len(calls) != 0 {
		t.Errorf("Serve off again: calls %q, want no serve at all", calls)
	}
	if r := row(); r.Detail != catalog.ServeOffDetail || r.URLs.Public != "http://127.0.0.1:40000" {
		t.Errorf("Serve off again: row %+v, want the admin step and the local URL", r)
	}
	for _, ip := range sockets.bound {
		if ip != "127.0.0.1" {
			t.Errorf("a listener bound %s, want 127.0.0.1 only", ip)
		}
	}
}

func TestARunRendersTheProfileEnvironmentAndAnAdoptionKeepsIt(t *testing.T) {
	kept := map[string]string{"EXPO_PACKAGER_PROXY_URL": "http://127.0.0.1:40000"}
	h := newHarness(t, machines.VMRecord{Name: "api", Lifetime: machines.Lasting, Want: machines.WantRunning, Env: kept})
	h.d.tn = &fakeTailnet{self: tailnet.Self{DNSName: "x.tail1234.ts.net", BackendState: "Running", ServeOn: true}}
	yes := true
	h.d.leases = []services.Lease{{VM: "api", GuestPort: catalog.Profiles[0].Port, HostPort: 40000, HTTP: &yes, Pinned: true, Profile: catalog.Profiles[0].Name}}
	h.d.refreshSelf(t.Context())
	h.run.Spawn(4242)
	_ = h.fs.WriteFile(h.d.pidFile("api"), []byte("4242\n"), 0o600)
	ctx := cancelled(t)
	h.d.adopt(ctx)
	if len(h.m.saved) != 0 {
		t.Fatalf("an adoption rewrote the record: %+v", h.m.saved)
	}
	v, _ := h.d.find("api")
	h.run.Exit(4242)
	_ = h.d.start(ctx, v)
	if len(h.m.saved) != 1 || h.m.saved[0].Env["EXPO_PACKAGER_PROXY_URL"] != "https://x.tail1234.ts.net:40000" || h.m.saved[0].Env["STATION_PUBLIC_HOST"] != "x.tail1234.ts.net" {
		t.Errorf("a run with Serve on saved %+v, want the https URL and the public host", h.m.saved)
	}
}

func certLines(d *Daemon) chan string {
	lines := make(chan string, 8)
	d.note = func(line string) {
		if strings.Contains(line, "certificate for") {
			lines <- line
		}
	}
	return lines
}

func TestTheCertificateIsWarmedWhenServeTurnsOnOneAtATime(t *testing.T) {
	h := newHarness(t)
	tn := &fakeTailnet{
		self: tailnet.Self{DNSName: "x.tail1234.ts.net", BackendState: "Running", ServeOn: true},
		hold: make(chan struct{}), entered: make(chan string, 4),
	}
	h.d.tn, h.d.h.Sockets = tn, &loopback{}
	ready := certLines(h.d)
	ctx := t.Context()
	yes := true
	h.d.leases = []services.Lease{{VM: "api", GuestPort: 8000, HostPort: 40000, HTTP: &yes}}
	h.d.refreshSelf(ctx)
	<-tn.entered
	h.d.mu.Lock()
	s := h.d.svcOf("api", 8000)
	s.st.Phase = services.Published
	h.d.publish(ctx, s)
	h.d.mu.Unlock()
	defer h.d.closeAll()
	if r := h.d.hostView("").Services[0]; r.Detail != issuingDetail || r.URLs.Public != "https://x.tail1234.ts.net:40000" {
		t.Fatalf("while the certificate is issued: row %+v, want the https URL with %q", r, issuingDetail)
	}
	tn.turn(false)
	h.d.step(ctx, serveEvery)
	tn.turn(true)
	h.d.step(ctx, 2*serveEvery)
	if got := tn.issued(); len(got) != 1 || got[0] != "x.tail1234.ts.net" {
		t.Fatalf("Serve on at start, then off and on during the warm-up: certs %q, want one for the host", got)
	}
	close(tn.hold)
	if line := <-ready; !strings.HasSuffix(line, "tailnet: certificate for x.tail1234.ts.net ready in 0s") {
		t.Errorf("daemon.log %q, want the ready line", line)
	}
	for tick := 2*serveEvery + 1; tick <= 3*serveEvery; tick++ {
		h.d.step(ctx, tick)
	}
	if got := tn.issued(); len(got) != 1 {
		t.Fatalf("Serve still on: certs %q, want no second warm-up", got)
	}
	if r := h.d.hostView("").Services[0]; r.Detail != "" {
		t.Fatalf("the certificate ready: row detail %q, want none", r.Detail)
	}
	tn.turn(false)
	h.d.step(ctx, 4*serveEvery)
	tn.turn(true)
	h.d.step(ctx, 5*serveEvery)
	<-tn.entered
	<-ready
	if got := tn.issued(); len(got) != 2 {
		t.Fatalf("Serve off, then on again: certs %q, want a second warm-up", got)
	}
}

func TestACertificateNeverIssuedIsCutAtItsBudgetAndSaysSo(t *testing.T) {
	h := newHarness(t)
	tn := &fakeTailnet{self: tailnet.Self{DNSName: "x.tail1234.ts.net", BackendState: "Running"}, certErr: context.DeadlineExceeded}
	h.d.tn = tn
	var lines []string
	done := make(chan struct{})
	h.d.note = func(line string) {
		if strings.Contains(line, "certificate for") {
			lines = append(lines, line)
			close(done)
		}
	}
	h.d.refreshSelf(t.Context())
	if got := tn.issued(); len(got) != 0 {
		t.Fatalf("Serve off at start: certs %q, want none", got)
	}
	tn.turn(true)
	h.d.step(t.Context(), serveEvery)
	<-done
	tn.mu.Lock()
	until := tn.until
	tn.mu.Unlock()
	if want := h.clock.At.Add(certBudget); !until.Equal(want) {
		t.Errorf("the warm-up's deadline: %s, want %s", until, want)
	}
	if len(lines) != 1 || !strings.HasSuffix(lines[0], "tailnet: certificate for x.tail1234.ts.net not issued: no answer within 2m0s") {
		t.Errorf("daemon.log %q, want the not-issued line", lines)
	}
}
