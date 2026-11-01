package daemon

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/domain/machines"
	"github.com/jterrazz/studio-station/src/domain/settings"
	"github.com/jterrazz/studio-station/src/system"
	"github.com/jterrazz/studio-station/src/system/systemtest"
)

type fakeMachines struct {
	records  []machines.VMRecord
	poweroff map[string]bool
	saved    []machines.VMRecord
}

func (f *fakeMachines) Records() ([]machines.VMRecord, error) { return f.records, nil }

func (f *fakeMachines) Save(r machines.VMRecord) error {
	f.saved = append(f.saved, r)
	return nil
}

func (f *fakeMachines) Remove(context.Context, string) (machines.DeleteReport, error) {
	return machines.DeleteReport{}, nil
}

func (f *fakeMachines) PowerOff(context.Context, string) error     { return nil }
func (f *fakeMachines) EngineRunning(context.Context, string) bool { return false }
func (f *fakeMachines) DeclaredPoweroff(console string) bool       { return f.poweroff[console] }
func (f *fakeMachines) KeyPath() string                            { return "/h/.ssh/id_ed25519" }
func (f *fakeMachines) VMUser() string                             { return "vmu" }
func (f *fakeMachines) HereName() string                           { return "x" }
func (f *fakeMachines) ConsolePath(name string) string             { return "/s/consoles/" + name }
func (f *fakeMachines) LogPath(name string) string                 { return "/s/logs/" + name + ".log" }
func (f *fakeMachines) KnownHostsPath(name string) string          { return "/s/known_hosts/" + name }

type fakeEngine struct{ system.Engine }

func (fakeEngine) RunArgv(name, console string, _ system.RunOptions) []string {
	return []string{"tart", "run", "--serial-path", console, name}
}

func (fakeEngine) IP(context.Context, string, time.Duration) (string, error) {
	return "", errors.New("no ip in a module test")
}

type harness struct {
	d     *Daemon
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
	m := &fakeMachines{records: records, poweroff: map[string]bool{}}
	paths := settings.Paths{Config: "/c", State: "/s"}
	d := New(h, fakeEngine{}, nil, m, nil, settings.NewStore(files, paths), Options{Paths: paths, Here: catalog.Host})
	for _, r := range records {
		run.On("sh", RunCmd(fakeEngine{}.RunArgv(r.Name, m.ConsolePath(r.Name), system.RunOptions{}), d.exitFile(r.Name)).Args...)
	}
	return harness{d: d, run: run, fs: files, clock: clock, m: m}
}

func (h harness) runs() int { return len(h.run.Calls) }

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
	_ = h.fs.WriteFile(h.d.pidFile("t"), []byte("77\n"), 0o600)
	h.d.adopt(cancelled(t))
	if got := h.state("t"); got != StateStale {
		t.Errorf("temporary t with a dead pid: %s, want stale", got)
	}
	if _, err := h.fs.ReadFile(h.d.pidFile("t")); err == nil {
		t.Errorf("the dead pid file of t was kept")
	}
	if got := h.state("api"); got != StateRunning || h.d.readPID("api") == 0 {
		t.Errorf("lasting api wanted running: %s pid %d, want a new run", got, h.d.readPID("api"))
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
	h.run.Spawn(4242)
	_ = h.fs.WriteFile(h.d.pidFile("api"), []byte("4242\n"), 0o600)
	h.d.adopt(cancelled(t))
	if got := h.state("api"); got != StateRunning || h.runs() != 0 {
		t.Errorf("adopted api: %s with %d runs, want running and none", got, h.runs())
	}
}

func TestALastingCrashRunsAgainAfterTheDelayOnly(t *testing.T) {
	h := newHarness(t, machines.VMRecord{Name: "api", Lifetime: machines.Lasting, Want: machines.WantRunning})
	ctx := cancelled(t)
	h.d.adopt(ctx)
	pid := h.d.readPID("api")
	h.run.Exit(pid)
	_ = h.fs.WriteFile(h.d.exitFile("api"), []byte("1\n"), 0o600)
	h.d.tick(ctx)
	if got := h.state("api"); got != StateStopped || h.runs() != 1 {
		t.Fatalf("after the crash: %s with %d runs, want stopped and no second run", got, h.runs())
	}
	h.clock.At = h.clock.At.Add(machines.CrashDelay - time.Second)
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
		h.run.Exit(h.d.readPID("t"))
		_ = h.fs.WriteFile(h.d.exitFile("t"), []byte("0\n"), 0o600)
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
