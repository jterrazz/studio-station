package remote_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jterrazz/studio-station/src/core/call"
	"github.com/jterrazz/studio-station/src/core/fail"
	"github.com/jterrazz/studio-station/src/core/machine"
	"github.com/jterrazz/studio-station/src/core/section"
	"github.com/jterrazz/studio-station/src/remote"
)

type installIn struct{ App string }

type installOut struct {
	App string
	// value: an error inside the result, which must reach the leaf as an error.
	Warning error
}

var (
	laptop = machine.Profile{Ref: machine.Ref{Name: "jterrazz-macbook-pro"}, Kind: machine.Mac, System: machine.System{OS: machine.MacOS}}
	infra  = machine.Profile{Ref: machine.Ref{Name: "infra", On: "jterrazz-mac-mini"}, Kind: machine.Server, System: machine.System{OS: machine.Linux}}
	linux1 = machine.Profile{Ref: machine.Ref{Name: "linux-1", On: "jterrazz-macbook-pro"}, Kind: machine.Workspace, System: machine.System{OS: machine.Linux}}
)

func onMacOS(p machine.Profile) machine.Verdict {
	if p.System.OS == machine.Linux {
		return machine.No(p.Ref.Name + " runs Linux: Mac App Store apps live on macOS only")
	}
	return machine.Yes
}

var errLate = errors.New("jq installed, its shell completion did not")

// value: an install in each Where, so a router that sends one Where elsewhere is seen.
func installs(ran *int) []call.Handler {
	run := func(_ context.Context, _ struct{}, in installIn, _ call.Progress) (installOut, error) {
		*ran++
		return installOut{App: in.App, Warning: errLate}, nil
	}
	out := make([]call.Handler, 0, 4)
	for _, where := range []call.Where{call.AtTarget, call.InEngine, call.InProcess} {
		out = append(out, call.Bind(call.Call[struct{}, installIn, installOut]{
			Name: "apps.install-" + string(where), Section: section.Apps, Where: where, Applies: onMacOS, Run: run,
		}, struct{}{}))
	}
	out = append(out, call.BindWatch(call.Watch[struct{}, installIn, installOut]{
		Name: "apps.list", Section: section.Apps, Where: call.AtTarget, Applies: onMacOS,
		Run: func(ctx context.Context, _ struct{}, _ installIn, _ call.Progress) (<-chan call.Update[installOut], error) {
			*ran++
			return call.Updates(ctx, make(chan installOut)), nil
		},
	}, struct{}{}))
	return out
}

func routerOn(t *testing.T, here machine.Profile, ran *int) *remote.Router {
	t.Helper()
	registry, err := call.NewRegistry(installs(ran))
	if err != nil {
		t.Fatal(err)
	}
	return remote.NewRouter(registry, here, remote.Reach{})
}

func TestEveryCallForThisMachineRunsInThisProcessWithItsValuesWhole(t *testing.T) {
	for _, where := range []call.Where{call.AtTarget, call.InEngine, call.InProcess} {
		for _, target := range []machine.Ref{{}, laptop.Ref} {
			ran := 0
			var out installOut
			err := routerOn(t, laptop, &ran).Invoke(t.Context(), target, "apps.install-"+string(where), installIn{App: "jq"}, &out, nil)
			if err != nil || ran != 1 || out.App != "jq" || !errors.Is(out.Warning, errLate) {
				t.Errorf("installing jq %s on %q: %+v, %v, ran %d times; want it run here once, its warning still an error", where, target.Name, out, err, ran)
			}
		}
	}
}

func TestACallRunInThisProcessPassesTheGateOfAFrame(t *testing.T) {
	for _, c := range []struct {
		gesture string
		here    machine.Profile
		want    string
	}{
		{"a server has no Apps section", infra, "apps"},
		{"Mac App Store apps on a Linux VM", linux1, "linux-1 runs Linux: Mac App Store apps live on macOS only"},
	} {
		t.Run(c.gesture, func(t *testing.T) {
			ran := 0
			r := routerOn(t, c.here, &ran)
			var out installOut
			err := r.Invoke(t.Context(), machine.Ref{}, "apps.install-in-process", installIn{App: "keynote"}, &out, nil)
			_, watchErr := r.Watch(t.Context(), machine.Ref{}, "apps.list", installIn{}, nil)
			for _, err := range []error{err, watchErr} {
				var e *fail.Error
				if !errors.As(err, &e) || e.Code != call.NotForThisMachine || (e.Detail["section"] != c.want && e.Detail["why"] != c.want) {
					t.Errorf("got %v, want not-for-this-machine saying %q", err, c.want)
				}
			}
			if ran != 0 {
				t.Errorf("the refused call ran %d times", ran)
			}
		})
	}
}
