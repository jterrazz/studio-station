package machines_test

import (
	"context"
	"errors"
	"testing"
	"time"

	corefake "github.com/jterrazz/studio-station/src/core/adapter/fake"
	model "github.com/jterrazz/studio-station/src/machines"
	machinesfake "github.com/jterrazz/studio-station/src/machines/adapter/fake"
	machinescontract "github.com/jterrazz/studio-station/src/machines/contract"
	setupcontract "github.com/jterrazz/studio-station/src/setup/contract"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/domain/machines"
	"github.com/jterrazz/studio-station/src/domain/settings"
	"github.com/jterrazz/studio-station/src/system"
	"github.com/jterrazz/studio-station/src/system/systemtest"
)

type noTailnet struct{}

func (noTailnet) Peers(context.Context) (map[string]machines.Node, error) {
	return nil, nil
}

func service(t *testing.T, run system.Runner) (*machines.Service, *systemtest.Clock) {
	t.Helper()
	files := systemtest.NewFS()
	clock := &systemtest.Clock{At: time.Unix(0, 0)}
	h := system.Host{Run: run, FS: files, Env: systemtest.Env{HomeDir: "/home"}, Net: systemtest.NewNet(), Clock: clock}
	paths := settings.Paths{Config: "/config", State: "/state"}
	prefs := corefake.NewPreferences()
	p := machines.Prefs{Name: setupcontract.Name(prefs), Hosts: machinescontract.Macs(prefs), VMUser: machinescontract.VMUser(prefs)}
	if err := p.Name.Set("mac"); err != nil {
		t.Fatal(err)
	}
	if err := p.Hosts.Set(map[string]model.MacRecord{"mac": {}, "x": {User: "u"}}); err != nil {
		t.Fatal(err)
	}
	return machines.New(h, nil, p, machinesfake.NewStore(), noTailnet{}, nil, nil, nil, machines.Options{Here: catalog.Mac, KnownHosts: paths.State + "/known_hosts", Logs: paths.State + "/logs", Unlocker: restarted{}}), clock
}

type restarted struct{}

func (restarted) Unlock(context.Context, string, string) error { return nil }
func (restarted) PlannedRestart(context.Context, string) error { return nil }

type answering struct {
	*systemtest.Runner
	polls []int
}

func (a *answering) Output(ctx context.Context, c system.Cmd) (system.Output, error) {
	if c.Name == "ssh" && c.Args[len(c.Args)-1] == "true" {
		code := a.polls[0]
		if len(a.polls) > 1 {
			a.polls = a.polls[1:]
		}
		return system.Output{ExitCode: code}, nil
	}
	return a.Runner.Output(ctx, c)
}

func restarting(t *testing.T, polls ...int) (*machines.Service, *systemtest.Clock) {
	t.Helper()
	return service(t, &answering{Runner: systemtest.NewRunner(t), polls: polls})
}

func TestRestartWaitsForTheHostToGoAwayAndComeBack(t *testing.T) {
	svc, _ := restarting(t, 0, 0, 255, 255, 0)
	var seen []string

	report, err := svc.Restart(context.Background(), "x", func(e machines.RestartEvent) { seen = append(seen, e.Phase) })

	if err != nil {
		t.Fatalf("Restart: %v", err)
	}
	if report.Waited != 6*time.Second {
		t.Fatalf("waited %v, want 6s (two polls still up, one down, one down, then up)", report.Waited)
	}
	if want := []string{"restarting", "down", "back"}; len(seen) != 3 || seen[1] != want[1] || seen[2] != want[2] {
		t.Fatalf("progress = %v, want %v", seen, want)
	}
}

func TestRestartDoesNotTakeAHostThatNeverWentDownForRestarted(t *testing.T) {
	svc, clock := restarting(t, 0)

	_, err := svc.Restart(context.Background(), "x", nil)

	var timeout *machines.RestartTimeoutError
	if !errors.As(err, &timeout) || timeout.Phase != "stop" {
		t.Fatalf("Restart error = %v, want a timeout in the stop phase", err)
	}
	if spent := clock.At.Sub(time.Unix(0, 0)); spent != time.Minute {
		t.Fatalf("spent %v before giving up, want the 60s stop budget", spent)
	}
}

func TestRestartGivesUpOnAHostThatNeverComesBack(t *testing.T) {
	svc, clock := restarting(t, 255)

	_, err := svc.Restart(context.Background(), "x", nil)

	var timeout *machines.RestartTimeoutError
	if !errors.As(err, &timeout) || timeout.Phase != "return" {
		t.Fatalf("Restart error = %v, want a timeout in the return phase", err)
	}
	if spent := clock.At.Sub(time.Unix(0, 0)); spent != 3*time.Minute {
		t.Fatalf("spent %v, want the 180s return budget", spent)
	}
}
