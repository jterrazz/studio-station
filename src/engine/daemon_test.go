package engine

import (
	"context"
	"errors"
	"net/netip"
	"sync"
	"testing"
	"time"

	model "github.com/jterrazz/studio-station/src/machines"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/domain/machines"
	"github.com/jterrazz/studio-station/src/domain/settings"
	"github.com/jterrazz/studio-station/src/system/systemtest"
)

type fakeMachines struct {
	stuck   map[string]bool
	records []machines.VMRecord
	saved   []machines.VMRecord
}

func (f *fakeMachines) Records() ([]machines.VMRecord, error) { return f.records, nil }

func (f *fakeMachines) Save(r machines.VMRecord) error {
	f.saved = append(f.saved, r)
	return nil
}

func (f *fakeMachines) Remove(context.Context, string) (machines.DeleteReport, error) {
	return machines.DeleteReport{}, nil
}

func (f *fakeMachines) PowerOff(context.Context, string) error            { return nil }
func (f *fakeMachines) EngineRunning(_ context.Context, name string) bool { return f.stuck[name] }
func (f *fakeMachines) HereName() string                                  { return "x" }

type fakeRun struct {
	vm    string
	pid   int
	ended chan model.RunEnd
}

func (r *fakeRun) VM() string                 { return r.vm }
func (r *fakeRun) PID() int                   { return r.pid }
func (r *fakeRun) Ended() <-chan model.RunEnd { return r.ended }

type fakeHypervisor struct {
	mu    sync.Mutex
	boots int
	live  map[string]*fakeRun
}

func newHypervisor() *fakeHypervisor { return &fakeHypervisor{live: map[string]*fakeRun{}} }

func (h *fakeHypervisor) Boot(_ context.Context, vm string, _ model.Network) (Run, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.boots++
	r := &fakeRun{vm: vm, pid: 1000 + h.boots, ended: make(chan model.RunEnd, 1)}
	h.live[vm] = r
	return r, nil
}

func (h *fakeHypervisor) Adopt(_ context.Context, vm string) (Run, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	r, ok := h.live[vm]
	if !ok {
		return nil, false
	}
	return r, true
}

// value: outlived starts a run of vm before the engine does, as one left by an earlier engine.
func (h *fakeHypervisor) outlived(vm string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.live[vm] = &fakeRun{vm: vm, pid: 4242, ended: make(chan model.RunEnd, 1)}
}

func (h *fakeHypervisor) end(vm string, end model.RunEnd) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if r, ok := h.live[vm]; ok {
		delete(h.live, vm)
		r.ended <- end
	}
}

func (h *fakeHypervisor) started() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.boots
}

func (*fakeHypervisor) Shutdown(context.Context, string, time.Duration) error { return nil }

func (*fakeHypervisor) Address(context.Context, string, time.Duration) (netip.Addr, error) {
	return netip.Addr{}, errors.New("no ip in a module test")
}

type harness struct {
	d     *Daemon
	hv    *fakeHypervisor
	run   *systemtest.Runner
	fs    *systemtest.FS
	clock *systemtest.Clock
	m     *fakeMachines
}

func newHarness(t *testing.T, records ...machines.VMRecord) harness {
	t.Helper()
	run, files := systemtest.NewRunner(t), systemtest.NewFS()
	clock := &systemtest.Clock{At: time.Date(2026, 11, 1, 10, 0, 0, 0, time.UTC)}
	h := systemtest.Host(run, files, systemtest.Env{}, systemtest.NewNet(), clock)
	m := &fakeMachines{records: records}
	hv := newHypervisor()
	paths := settings.Paths{Config: "/c", State: "/s"}
	d := New(h, hv, nil, m, &fakeTailnet{}, memStores(), Options{State: paths.State, Here: catalog.Host})
	return harness{d: d, hv: hv, run: run, fs: files, clock: clock, m: m}
}

func (h harness) runs() int { return h.hv.started() }

func (h harness) state(name string) string {
	v, err := h.d.find(name)
	if err != nil {
		return "unknown"
	}
	h.d.mu.Lock()
	defer h.d.mu.Unlock()
	return v.state
}

func cancelled(t *testing.T) context.Context {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	return ctx
}

func TestAtStartADeadRunLeavesATemporaryVMStaleAndRunsALastingOneItsRecordWants(t *testing.T) {
	h := newHarness(t,
		machines.VMRecord{Name: "t", Lifetime: machines.Temporary},
		machines.VMRecord{Name: "api", Lifetime: machines.Lasting, Want: machines.WantRunning},
		machines.VMRecord{Name: "off", Lifetime: machines.Lasting, Want: machines.WantStopped},
	)
	h.d.adopt(cancelled(t))
	if got := h.state("t"); got != StateStale {
		t.Errorf("temporary t with no live run: %s, want stale", got)
	}
	if got := h.state("api"); got != StateRunning {
		t.Errorf("lasting api wanted running: %s, want a new run", got)
	}
	if got := h.state("off"); got != StateStopped {
		t.Errorf("lasting off wanted stopped: %s", got)
	}
	if h.runs() != 1 {
		t.Errorf("%d runs started, want only api's", h.runs())
	}
}

func TestAtStartALiveRunIsAdoptedNeverRunAgain(t *testing.T) {
	h := newHarness(t, machines.VMRecord{Name: "api", Lifetime: machines.Lasting, Want: machines.WantRunning})
	h.hv.outlived("api")
	h.d.adopt(cancelled(t))
	if got := h.state("api"); got != StateRunning || h.runs() != 0 {
		t.Errorf("adopted api: %s with %d runs, want running and none", got, h.runs())
	}
}

func TestALastingCrashRunsAgainAfterTheDelayOnly(t *testing.T) {
	h := newHarness(t, machines.VMRecord{Name: "api", Lifetime: machines.Lasting, Want: machines.WantRunning})
	ctx := cancelled(t)
	h.d.adopt(ctx)
	h.hv.end("api", model.RunEnd{Reason: model.Crashed, ExitCode: 1})
	h.d.tick(ctx)
	if got := h.state("api"); got != StateStopped || h.runs() != 1 {
		t.Fatalf("after the crash: %s with %d runs, want stopped and no second run", got, h.runs())
	}
	h.clock.At = h.clock.At.Add(model.CrashDelay - time.Second)
	h.d.tick(ctx)
	if h.runs() != 1 {
		t.Fatalf("a run started before the delay")
	}
	h.clock.At = h.clock.At.Add(time.Second)
	h.d.tick(ctx)
	if got := h.state("api"); got != StateRunning || h.runs() != 2 {
		t.Errorf("after the delay: %s with %d runs, want running and a second run", got, h.runs())
	}
}

func TestAFourthStopWithinAMinuteLeavesTheVMBroken(t *testing.T) {
	h := newHarness(t, machines.VMRecord{Name: "t", Lifetime: machines.Temporary})
	ctx := cancelled(t)
	v, _ := h.d.find("t")
	_ = h.d.start(ctx, v)
	for i := range 4 {
		h.hv.end("t", model.RunEnd{Reason: model.GuestReboot})
		h.clock.At = h.clock.At.Add(5 * time.Second)
		h.d.tick(ctx)
		if i < 3 && h.state("t") != StateRunning {
			t.Fatalf("stop %d: %s, want a re-run", i+1, h.state("t"))
		}
	}
	if got := h.state("t"); got != StateBroken || h.runs() != 4 {
		t.Errorf("after 4 stops in 20 s: %s with %d runs, want broken and no fifth run", got, h.runs())
	}
}

func TestARunWhoseGuestNeverConnectsAnswersGuestTimeoutAfterAMinute(t *testing.T) {
	h := newHarness(t, machines.VMRecord{Name: "api", Lifetime: machines.Lasting, Want: machines.WantRunning})
	h.hv.outlived("api")
	v, _ := h.d.find("api")
	v.run, _ = h.hv.Adopt(t.Context(), "api")
	v.state, v.guest = StateRunning, GuestConnecting
	began := h.clock.At
	err := h.d.awaitGuest(t.Context(), "api")
	var op *OpError
	if !errors.As(err, &op) || op.Code != "guest_timeout" {
		t.Fatalf("a guest that never connects: %v, want guest_timeout", err)
	}
	if waited := h.clock.At.Sub(began); waited < guestReady {
		t.Errorf("guest_timeout after %s, want %s", waited, guestReady)
	}
}

func TestStopAllAnswersFailedForALastingVMTheEngineStillRuns(t *testing.T) {
	h := newHarness(t,
		machines.VMRecord{Name: "api", Lifetime: machines.Lasting, Want: machines.WantRunning},
		machines.VMRecord{Name: "web", Lifetime: machines.Lasting, Want: machines.WantRunning},
	)
	h.m.stuck = map[string]bool{"api": true}
	for _, name := range []string{"api", "web"} {
		v, _ := h.d.find(name)
		v.state = StateRunning
	}
	got := h.d.stopAll(t.Context())
	want := []Stopped{
		{Name: "api", Lifetime: "lasting", Result: ResultFailed, Detail: "the engine still runs it"},
		{Name: "web", Lifetime: "lasting", Result: ResultStopped},
	}
	if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("stop_all answered %+v, want %+v", got, want)
	}
}
