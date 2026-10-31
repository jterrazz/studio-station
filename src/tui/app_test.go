package tui

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/domain/clean"
	"github.com/jterrazz/studio-station/src/domain/config"
	"github.com/jterrazz/studio-station/src/domain/images"
	"github.com/jterrazz/studio-station/src/domain/machines"
	"github.com/jterrazz/studio-station/src/domain/secrets"
	"github.com/jterrazz/studio-station/src/domain/status"
	"github.com/jterrazz/studio-station/src/domain/tailnet"
	"github.com/jterrazz/studio-station/src/domain/tools"
	"github.com/jterrazz/studio-station/src/theme"
	"github.com/jterrazz/studio-station/src/tui/kit"
	cleanscreen "github.com/jterrazz/studio-station/src/tui/screens/clean"
	configscreen "github.com/jterrazz/studio-station/src/tui/screens/config"
	fleetscreen "github.com/jterrazz/studio-station/src/tui/screens/fleet"
	statusscreen "github.com/jterrazz/studio-station/src/tui/screens/status"
	toolsscreen "github.com/jterrazz/studio-station/src/tui/screens/tools"
)

type fakeTools struct {
	inventories int
	failLoad    error
	failInstall error
	installed   []tools.InstallRequest
}

func (f *fakeTools) Inventory(context.Context) ([]tools.Report, error) {
	f.inventories++
	if f.failLoad != nil {
		return nil, f.failLoad
	}
	return []tools.Report{
		{Tool: catalog.Tool{Name: "homebrew", Category: catalog.CategoryPackageManagers}, Method: catalog.MethodScript, State: catalog.StateOK, Version: "5.0.1"},
		{Tool: catalog.Tool{Name: "node", Category: catalog.CategoryRuntimes}, Method: catalog.MethodMise, State: catalog.StateDegraded, Version: "24.11.0", Detail: "on PATH, not managed by mise"},
		{Tool: catalog.Tool{Name: "go", Category: catalog.CategoryRuntimes}, Method: catalog.MethodMise, State: catalog.StateOK, Version: "1.27.3"},
		{Tool: catalog.Tool{Name: "java", Category: catalog.CategoryRuntimes}, Method: catalog.MethodMise, State: catalog.StateAbsent},
	}, nil
}

func (f *fakeTools) CheckInstall(context.Context, []string) error { return nil }

func (f *fakeTools) Install(_ context.Context, r tools.InstallRequest) ([]tools.Outcome, error) {
	f.installed = append(f.installed, r)
	if f.failInstall != nil {
		return nil, f.failInstall
	}
	return []tools.Outcome{{Name: r.Names[0], Verb: "install", Result: catalog.StateOK, Detail: "21.0.5"}}, nil
}

func (f *fakeTools) CheckUninstall([]string) error { return nil }

func (f *fakeTools) Uninstall(context.Context, []string) ([]tools.Outcome, error) { return nil, nil }

func (f *fakeTools) CheckUpgrade([]string) error { return nil }

func (f *fakeTools) Upgrade(context.Context, tools.UpgradeRequest) ([]tools.Outcome, error) {
	return nil, nil
}

type fakeConfig struct{}

func (fakeConfig) List(context.Context) ([]config.Report, error) {
	return []config.Report{{Item: catalog.ConfigItem{Name: "hushlogin", Description: "No login banner", Category: catalog.CategoryShell}, State: catalog.StateAbsent, Applies: true}}, nil
}

func (fakeConfig) Defaults(string) map[string]string { return nil }
func (fakeConfig) CheckInstall([]string, bool) error { return nil }
func (fakeConfig) CheckUninstall([]string) error     { return nil }
func (fakeConfig) Uninstall(context.Context, string) (config.Change, error) {
	return config.Change{}, nil
}

func (fakeConfig) Install(context.Context, string, map[string]string, bool) (config.Change, error) {
	return config.Change{}, nil
}

type fakeClean struct{}

func (fakeClean) List(context.Context) ([]clean.Report, error) {
	return []clean.Report{{Item: catalog.Cleanable{Name: "brew", Description: "The Homebrew download cache"}, Available: true, Bytes: 1_200_000_000}}, nil
}

func (fakeClean) Clean(context.Context, []string) ([]clean.Outcome, error) { return nil, nil }

type fakeStatus struct{}

func (fakeStatus) Snapshot(context.Context) (status.Snapshot, error) {
	return status.Snapshot{Machine: status.Machine{Name: "macbook"}}, nil
}

func (fakeStatus) Sample(context.Context) (status.Sample, error) { return status.Sample{}, nil }

type fakeFleet struct{ calls []string }

func (f *fakeFleet) Fleet(context.Context, bool) (machines.Fleet, error) {
	return machines.Fleet{Hosts: []machines.HostView{
		{Name: "macbook", Here: true, Reachable: true, VMs: []machines.VM{
			{Name: "linux-1", Kind: catalog.Linux, Host: "macbook", Lifetime: machines.Temporary, State: machines.Running},
			{Name: "linux-2", Kind: catalog.Linux, Host: "macbook", Lifetime: machines.Lasting, State: machines.Stopped},
		}},
		{Name: "mac-mini", Reachable: true},
	}}, nil
}

func (f *fakeFleet) record(call string) error {
	f.calls = append(f.calls, call)
	return nil
}

func (f *fakeFleet) SSHBlocks(context.Context, machines.Fleet) error { return nil }
func (f *fakeFleet) CheckCreate(_ context.Context, r machines.CreateRequest) (machines.CreateRequest, error) {
	return r, nil
}

func (f *fakeFleet) Create(_ context.Context, r machines.CreateRequest, _ func(machines.Step)) (machines.VM, error) {
	return machines.VM{Name: r.Name, Host: "macbook"}, f.record("create " + r.Name)
}
func (f *fakeFleet) Start(_ context.Context, name string) error { return f.record("start " + name) }
func (f *fakeFleet) Stop(_ context.Context, name string) error  { return f.record("stop " + name) }
func (f *fakeFleet) Delete(_ context.Context, name string) (machines.DeleteReport, error) {
	return machines.DeleteReport{Name: name}, f.record("delete " + name)
}
func (f *fakeFleet) ForgetHost(name string) error { return f.record("forget " + name) }
func (f *fakeFleet) Probe(_ context.Context, name string) (machines.ProbeReport, error) {
	return machines.ProbeReport{Name: name}, nil
}

func (f *fakeFleet) Restart(_ context.Context, name string, _ func(machines.RestartEvent)) (machines.RestartReport, error) {
	return machines.RestartReport{Name: name}, f.record("restart " + name)
}
func (f *fakeFleet) Unlock(_ context.Context, name string) error { return f.record("unlock " + name) }

type fakeImages struct{}

func (fakeImages) List(context.Context) ([]images.Report, error) { return nil, nil }
func (fakeImages) Build(context.Context, images.BuildRequest, func(images.Step)) ([]images.Record, error) {
	return nil, nil
}

type fakeSecrets struct{}

func (fakeSecrets) List(context.Context) (secrets.Report, error) { return secrets.Report{}, nil }

type fakeTailnet struct{}

func (fakeTailnet) Status(context.Context) (tailnet.Status, error) {
	return tailnet.Status{State: catalog.StateOK, Connected: true}, nil
}
func (fakeTailnet) Up(context.Context) (tailnet.UpResult, error) { return tailnet.UpResult{}, nil }
func (fakeTailnet) Down(context.Context) (tailnet.DownResult, error) {
	return tailnet.DownResult{}, nil
}

type harness struct {
	t     *testing.T
	app   App
	tools *fakeTools
	fleet *fakeFleet
}

func describe(err error) (message, hint string) { return err.Error(), "run station --help" }

func newHarness(t *testing.T) *harness {
	t.Helper()
	ctx := context.Background()
	th := theme.New(true)
	h := &harness{t: t, tools: &fakeTools{}, fleet: &fakeFleet{}}
	h.app = newApp("macbook · host", Tabs(), []kit.Screen{
		statusscreen.New(ctx, fakeStatus{}, describe, th),
		toolsscreen.New(ctx, h.tools, describe, th),
		configscreen.New(ctx, fakeConfig{}, describe, th),
		cleanscreen.New(ctx, fakeClean{}, describe, th),
		fleetscreen.New(ctx, fleetscreen.Deps{
			Machines: h.fleet, Images: fakeImages{}, Secrets: fakeSecrets{}, Tailnet: fakeTailnet{},
			Remote:   func(_ context.Context, machine string, _ []string) error { return h.fleet.record("remote " + machine) },
			Describe: describe,
		}, th),
	})
	h.send(tea.WindowSizeMsg{Width: 100, Height: 30})
	return h
}

// value: a command is run the way the program would, but a tick or a blink that
// sleeps is dropped, so a test never waits on an animation.
func messages(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	got := make(chan tea.Msg, 1)
	go func() { got <- cmd() }()
	var msg tea.Msg
	select {
	case msg = <-got:
	case <-time.After(50 * time.Millisecond):
		return nil
	}
	if batch, ok := msg.(tea.BatchMsg); ok {
		var out []tea.Msg
		for _, c := range batch {
			out = append(out, messages(c)...)
		}
		return out
	}
	if v := reflect.ValueOf(msg); v.Kind() == reflect.Slice && v.Type().Elem() == reflect.TypeFor[tea.Cmd]() {
		var out []tea.Msg
		for i := range v.Len() {
			c, _ := v.Index(i).Interface().(tea.Cmd)
			out = append(out, messages(c)...)
		}
		return out
	}
	if msg == nil {
		return nil
	}
	return []tea.Msg{msg}
}

// value: send feeds one message and drains every command it returns, but stops at a
// terminal hand-over: the ExecMsg is returned, not run, so a test decides.
func (h *harness) send(msg tea.Msg) (execs []kit.ExecMsg, quit bool) {
	h.t.Helper()
	queue := []tea.Msg{msg}
	for len(queue) > 0 {
		next := queue[0]
		queue = queue[1:]
		if e, ok := next.(kit.ExecMsg); ok {
			execs = append(execs, e)
			continue
		}
		if _, ok := next.(tea.QuitMsg); ok {
			quit = true
			continue
		}
		model, cmd := h.app.Update(next)
		app, ok := model.(App)
		if !ok {
			h.t.Fatalf("Update returned %T", model)
		}
		h.app = app
		queue = append(queue, messages(cmd)...)
	}
	return execs, quit
}

func (h *harness) init() {
	h.t.Helper()
	for _, m := range messages(h.app.Init()) {
		h.send(m)
	}
}

func (h *harness) press(keys ...string) (execs []kit.ExecMsg, quit bool) {
	h.t.Helper()
	for _, k := range keys {
		e, q := h.send(keyOf(k))
		execs, quit = append(execs, e...), quit || q
	}
	return execs, quit
}

func (h *harness) typeText(s string) {
	for _, r := range s {
		h.send(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
}

func keyOf(k string) tea.KeyPressMsg {
	switch k {
	case "tab":
		return tea.KeyPressMsg{Code: tea.KeyTab}
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	case "esc":
		return tea.KeyPressMsg{Code: tea.KeyEscape}
	case "space":
		return tea.KeyPressMsg{Code: tea.KeySpace, Text: " "}
	case "down":
		return tea.KeyPressMsg{Code: tea.KeyDown}
	case "ctrl+r":
		return tea.KeyPressMsg{Code: 'r', Mod: tea.ModCtrl}
	}
	r, _ := utf8.DecodeRuneInString(k)
	return tea.KeyPressMsg{Code: r, Text: k}
}

func (h *harness) view() string { return h.app.View().Content }

func (h *harness) contains(want ...string) {
	h.t.Helper()
	v := plain(h.view())
	for _, w := range want {
		if !strings.Contains(v, w) {
			h.t.Fatalf("view lacks %q:\n%s", w, v)
		}
	}
}

func (h *harness) lacks(unwanted ...string) {
	h.t.Helper()
	v := plain(h.view())
	for _, w := range unwanted {
		if strings.Contains(v, w) {
			h.t.Fatalf("view still shows %q:\n%s", w, v)
		}
	}
}

func plain(s string) string {
	var b strings.Builder
	escaping := false
	for i := range len(s) {
		switch c := s[i]; {
		case c == 0x1b:
			escaping = true
		case escaping && c >= '@' && c <= '~' && c != '[':
			escaping = false
		case !escaping:
			b.WriteByte(c)
		}
	}
	return b.String()
}

// value: cursorTo moves down until the cursor ROW (never a group title) holds name.
func (h *harness) cursorTo(name string) {
	h.t.Helper()
	for range 40 {
		for _, line := range strings.Split(h.app.View().Content, "\n") {
			text := plain(line)
			isHeader := strings.HasPrefix(text, " - ") || strings.HasPrefix(text, " + ")
			if strings.Contains(line, "48;2;") && !isHeader && strings.Contains(text, " "+name+" ") {
				return
			}
		}
		h.press("down")
	}
	h.t.Fatalf("no row %q under the cursor", name)
}

func TestFleetStartsALastingVMThroughExec(t *testing.T) {
	h := newHarness(t)
	h.init()
	h.press("5")
	h.cursorTo("linux-2")
	execs, _ := h.press("s")
	if len(execs) != 1 {
		t.Fatalf("s on a lasting VM ran %d actions", len(execs))
	}
	h.finish(execs[0])
	if got := strings.Join(h.fleet.calls, ","); got != "start linux-2" {
		t.Fatalf("calls %q", got)
	}
}

func TestTabsRestartAsksTheMachineName(t *testing.T) {
	h := newHarness(t)
	h.init()
	h.press("5")
	h.cursorTo("mac-mini")
	if execs, _ := h.press("R"); len(execs) != 0 || h.app.screens[4].Focus() != kit.FocusNormal {
		t.Fatal("R is bound")
	}
	h.press("ctrl+r")
	if h.app.screens[4].Focus() != kit.FocusModal {
		t.Fatal("ctrl+r opened no confirm")
	}
	h.contains("Restart mac-mini?")
	h.typeText("mac")
	if execs, _ := h.press("enter"); len(execs) != 0 {
		t.Fatal("a partial name restarted the host")
	}
	h.typeText("-mini")
	execs, _ := h.press("enter")
	if len(execs) != 1 {
		t.Fatalf("the typed name ran %d actions", len(execs))
	}
	h.finish(execs[0])
	if got := strings.Join(h.fleet.calls, ","); got != "restart mac-mini" {
		t.Fatalf("calls %q", got)
	}
}

func (h *harness) finish(e kit.ExecMsg) {
	h.t.Helper()
	err := e.Command()
	h.send(e.Done(err))
}

func TestLoadingIsInInitNeverInView(t *testing.T) {
	h := newHarness(t)
	for i, s := range h.app.screens {
		if s.Init() == nil {
			t.Fatalf("tab %d loads nothing in Init", i+1)
		}
	}
	h.contains("reading this machine")
	h.press("2")
	h.contains("loading")
	if h.tools.inventories != 0 {
		t.Fatal("rendering read the catalogue")
	}
	h.init()
	h.contains("Runtimes", "node", "java")
	if h.tools.inventories != 1 {
		t.Fatalf("the catalogue was read %d times", h.tools.inventories)
	}
}

func TestFoldStartsFoldedWhenAllOKAndToggles(t *testing.T) {
	h := newHarness(t)
	h.init()
	h.press("2")
	h.contains("+ Package Managers", "- Runtimes")
	h.lacks("homebrew")
	h.press("space")
	h.contains("- Package Managers", "homebrew")
}

func TestBusyIgnoresActionKeys(t *testing.T) {
	h := newHarness(t)
	h.init()
	h.press("2")
	h.cursorTo("java")
	h.press("i")
	execs, _ := h.press("y")
	h.press("i")
	if h.app.screens[1].Focus() != kit.FocusNormal {
		t.Fatal("an action key opened a modal while busy")
	}
	h.contains("busy")
	h.finish(execs[0])
	h.press("i")
	h.contains("Install java?")
}

func TestRetryAfterAFailedReadLoadsTheTab(t *testing.T) {
	h := newHarness(t)
	h.tools.failLoad = errors.New("brew is not installed")
	h.init()
	h.press("2")
	h.contains("press r to retry")
	h.tools.failLoad = nil
	h.press("r")
	h.contains("Runtimes")
	h.lacks("press r to retry")
}

func TestStatusBarShowsAFailedActionAndClearsOnTheNextKey(t *testing.T) {
	h := newHarness(t)
	h.tools.failInstall = errors.New("mise exited 1")
	h.init()
	h.press("2")
	h.cursorTo("java")
	h.press("i")
	execs, _ := h.press("y")
	h.finish(execs[0])
	h.contains("mise exited 1  run station --help")
	h.press("down")
	h.lacks("mise exited 1")
}
