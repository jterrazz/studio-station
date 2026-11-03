package call_test

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"testing"

	"github.com/jterrazz/studio-station/src/core/call"
	"github.com/jterrazz/studio-station/src/core/confirm"
	"github.com/jterrazz/studio-station/src/core/fail"
	"github.com/jterrazz/studio-station/src/core/machine"
	"github.com/jterrazz/studio-station/src/core/section"
)

type deps struct{ installed map[string]bool }

type installIn struct{ App string }

type installOut struct{ App, State string }

type removeIn struct{ App, Confirm string }

type listOut struct{ Apps []string }

var (
	laptop = machine.Profile{Ref: machine.Ref{Name: "jterrazz-macbook-pro"}, Kind: machine.Mac, Caps: machine.Capabilities{Battery: true, Lid: true, Engine: true}}
	infra  = machine.Profile{Ref: machine.Ref{Name: "infra", On: "jterrazz-mac-mini"}, Kind: machine.Server}
	linux1 = machine.Profile{Ref: machine.Ref{Name: "linux-1", On: "jterrazz-macbook-pro"}, Kind: machine.Workspace, System: machine.System{OS: machine.Linux}}
)

func onMacOS(p machine.Profile) machine.Verdict {
	if p.System.OS == machine.Linux {
		return machine.No(p.Ref.Name + " runs Linux: Mac App Store apps live on macOS only")
	}
	return machine.Yes
}

func appsSection(machine.Profile) machine.Verdict { return machine.Yes }

func install(ran *int) call.Call[deps, installIn, installOut] {
	return call.Call[deps, installIn, installOut]{
		Name: "apps.install", Section: section.Apps, Where: call.AtTarget, Applies: machine.All(appsSection, onMacOS),
		Run: func(_ context.Context, d deps, in installIn, p call.Progress) (installOut, error) {
			*ran++
			p.Step(in.App, "installing")
			d.installed[in.App] = true
			p.Done(in.App, nil)
			return installOut{App: in.App, State: "installed"}, nil
		},
	}
}

var remove = call.Call[deps, removeIn, struct{}]{
	Name: "apps.remove", Section: section.Apps, Where: call.AtTarget, Applies: appsSection, Confirm: confirm.Remove,
	Errors: []fail.Code{confirm.YesNeeded},
	Run: func(_ context.Context, d deps, in removeIn, _ call.Progress) (struct{}, error) {
		if err := confirm.Require(confirm.For(confirm.Remove, machine.Mac), in.App, in.Confirm); err != nil {
			return struct{}{}, err
		}
		delete(d.installed, in.App)
		return struct{}{}, nil
	},
}

var list = call.Watch[deps, struct{}, listOut]{
	Name: "apps.list", Section: section.Apps, Where: call.AtTarget, Applies: appsSection,
	Run: func(_ context.Context, _ deps, _ struct{}, _ call.Progress) (<-chan call.Update[listOut], error) {
		updates := make(chan call.Update[listOut], 2)
		updates <- call.Update[listOut]{Value: listOut{Apps: []string{"jq"}}}
		updates <- call.Update[listOut]{Value: listOut{Apps: []string{"jq", "node"}}}
		close(updates)
		return updates, nil
	},
}

type steps []string

func (s *steps) Step(task, detail string)  { *s = append(*s, task+" "+detail) }
func (s *steps) Done(task string, _ error) { *s = append(*s, task+" done") }

type router struct {
	registry *call.Registry
	machines map[string]machine.Profile
}

func (r router) Invoke(ctx context.Context, target machine.Ref, name string, in, out any, p call.Progress) error {
	h, _ := r.registry.Lookup(name)
	raw, err := json.Marshal(in)
	if err != nil {
		return err
	}
	res, err := h.Invoke(ctx, r.machines[target.Name], raw, p)
	if err != nil {
		return err
	}
	return json.Unmarshal(res, out)
}

func (r router) Watch(ctx context.Context, target machine.Ref, name string, in any) (<-chan call.Update[json.RawMessage], error) {
	h, _ := r.registry.Lookup(name)
	raw, err := json.Marshal(in)
	if err != nil {
		return nil, err
	}
	return h.Watch(ctx, r.machines[target.Name], raw, nil)
}

func notForThisMachine(t *testing.T, err error) *fail.Error {
	t.Helper()
	var e *fail.Error
	if !errors.As(err, &e) || e.Type != fail.NotForThisMachine || e.Code != call.NotForThisMachine || fail.ExitOf(err) != fail.ExitUsage {
		t.Fatalf("got %v, want not-for-this-machine, exit 2", err)
	}
	return e
}

func TestASurfaceInstallsAnAppThroughTheRouterWithoutSeeingItsDeps(t *testing.T) {
	ran, installed := 0, map[string]bool{}
	d := deps{installed: installed}
	registry, err := call.NewRegistry([]call.Handler{call.Bind(install(&ran), d), call.Bind(remove, d)})
	if err != nil {
		t.Fatal(err)
	}
	r := router{registry: registry, machines: map[string]machine.Profile{laptop.Ref.Name: laptop}}
	var progress steps

	out, err := call.Invoke(t.Context(), r, laptop.Ref, install(&ran), installIn{App: "jq"}, &progress)

	if err != nil || out != (installOut{App: "jq", State: "installed"}) || !installed["jq"] {
		t.Fatalf("installing jq on This Mac: %+v, %v", out, err)
	}
	if !slices.Equal(progress, steps{"jq installing", "jq done"}) {
		t.Errorf("the Progress strip showed %v", progress)
	}
}

func TestAConfirmationTheUseCaseAsksTravelsBackAsItsError(t *testing.T) {
	d := deps{installed: map[string]bool{"lazygit": true}}
	h := call.Bind(remove, d)

	_, err := h.Invoke(t.Context(), laptop, json.RawMessage(`{"App":"lazygit"}`), nil)

	var e *fail.Error
	if !errors.As(err, &e) || e.Code != confirm.YesNeeded || !d.installed["lazygit"] {
		t.Fatalf("removing lazygit without --yes: %v, still installed %v", err, d.installed["lazygit"])
	}
	if _, err := h.Invoke(t.Context(), laptop, json.RawMessage(`{"App":"lazygit","Confirm":"yes"}`), nil); err != nil || d.installed["lazygit"] {
		t.Errorf("removing lazygit with --yes: %v", err)
	}
}

func TestACallOfASectionAServerHasNotIsRefusedBeforeItRuns(t *testing.T) {
	ran := 0
	_, err := call.Bind(install(&ran), deps{installed: map[string]bool{}}).Invoke(t.Context(), infra, json.RawMessage(`{"App":"jq"}`), nil)

	e := notForThisMachine(t, err)
	if e.Subject != "infra" || e.Detail["section"] != "apps" || e.Detail["kind"] != "server" || ran != 0 {
		t.Errorf("installing on infra: %+v, ran %d times", e, ran)
	}
}

func TestACallItsRuleRefusesSaysWhyBeforeItRuns(t *testing.T) {
	ran := 0
	_, err := call.Bind(install(&ran), deps{installed: map[string]bool{}}).Invoke(t.Context(), linux1, json.RawMessage(`{"App":"keynote"}`), nil)

	e := notForThisMachine(t, err)
	if e.Detail["why"] != "linux-1 runs Linux: Mac App Store apps live on macOS only" || ran != 0 {
		t.Errorf("installing keynote on linux-1: %+v, ran %d times", e, ran)
	}
}

func TestAnInputThatDoesNotDecodeIsRefusedBeforeItRuns(t *testing.T) {
	ran := 0
	_, err := call.Bind(install(&ran), deps{installed: map[string]bool{}}).Invoke(t.Context(), laptop, json.RawMessage(`{"App":7}`), nil)

	var e *fail.Error
	if !errors.As(err, &e) || e.Type != fail.Refused || e.Code != call.BadInput || e.Cause == nil || fail.ExitOf(err) != fail.ExitUsage || ran != 0 {
		t.Errorf("a frame with a number for an app: %v, ran %d times", err, ran)
	}
}

func TestAWatchFillsTheListUpdateByUpdate(t *testing.T) {
	ran := 0
	d := deps{installed: map[string]bool{}}
	registry, err := call.NewRegistry([]call.Handler{call.BindWatch(list, d)}, []call.Handler{call.Bind(install(&ran), d)})
	if err != nil {
		t.Fatal(err)
	}
	r := router{registry: registry, machines: map[string]machine.Profile{laptop.Ref.Name: laptop}}

	updates, err := r.Watch(t.Context(), laptop.Ref, "apps.list", nil)
	if err != nil {
		t.Fatal(err)
	}
	var seen []string
	for u := range updates {
		seen = append(seen, string(u.Value))
	}
	if !slices.Equal(seen, []string{`{"Apps":["jq"]}`, `{"Apps":["jq","node"]}`}) {
		t.Errorf("apps.list streamed %v", seen)
	}

	var last listOut
	if failed := r.Invoke(t.Context(), laptop.Ref, "apps.list", nil, &last, nil); failed != nil || !slices.Equal(last.Apps, []string{"jq", "node"}) {
		t.Errorf("apps.list invoked once answered %+v, %v", last, failed)
	}
	once, err := r.Watch(t.Context(), laptop.Ref, "apps.install", installIn{App: "jq"})
	if u := <-once; err != nil || string(u.Value) != `{"App":"jq","State":"installed"}` {
		t.Errorf("apps.install watched answered %s, %v", u.Value, err)
	}
}

func TestTwoCallsCannotClaimOneName(t *testing.T) {
	ran := 0
	d := deps{installed: map[string]bool{}}
	if _, err := call.NewRegistry([]call.Handler{call.Bind(install(&ran), d)}, []call.Handler{call.Bind(install(&ran), d)}); err == nil {
		t.Error("two families both registered apps.install")
	}
}
