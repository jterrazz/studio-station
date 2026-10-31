package setup_test

import (
	"context"
	"testing"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/domain/settings"
	"github.com/jterrazz/studio-station/src/domain/setup"
	"github.com/jterrazz/studio-station/src/system"
	"github.com/jterrazz/studio-station/src/system/systemtest"
)

type registry struct{ here string }

func (r *registry) Here() (string, bool)       { return r.here, r.here != "" }
func (r *registry) Register(name string) error { r.here = name; return nil }

type daemon struct{ started, stopped int }

func (d *daemon) Running(context.Context) (bool, error) { return false, nil }
func (d *daemon) Stop(context.Context) error            { d.stopped++; return nil }
func (d *daemon) Start(context.Context) error           { d.started++; return nil }

func build(kind catalog.MachineKind, detached bool, reg *registry, d *daemon, run *systemtest.Runner, files *systemtest.FS) *setup.Service {
	paths := settings.Paths{Config: "/home/u/.config/station", State: "/home/u/.local/state/station"}
	h := systemtest.Host(run, files, systemtest.Env{HomeDir: "/home/u", Login: "u", Host: "Jean-Baptiste's MacBook Pro.local", OS: "darwin"}, systemtest.NewNet(), &systemtest.Clock{})
	return setup.New(h, settings.NewStore(files, paths), settings.NewKeychain(run).On(kind), reg, d,
		setup.Options{Here: kind, Paths: paths, Executable: "/home/u/.local/bin/station", Detached: detached})
}

func TestTheDetachedTripleStopsTheLiveDaemonMovesItsIdentityAndRestartsIt(t *testing.T) {
	run, files := systemtest.NewRunner(t), systemtest.NewFS()
	run.Live[4242] = true
	_ = files.WriteFile("/home/u/.studio/tailscale/tailscaled.pid", []byte("4242\n"), 0o600)
	_ = files.WriteFile("/home/u/.studio/tailscale/tailscaled.state", []byte("node-identity"), 0o600)
	d := &daemon{}

	report, err := build(catalog.Host, true, &registry{here: "macbook"}, d, run, files).Run(context.Background(), setup.Request{})

	if err != nil {
		t.Fatal(err)
	}
	moved, readErr := files.ReadFile("/home/u/.local/state/station/tailscaled/tailscaled.state")
	if readErr != nil || string(moved) != "node-identity" {
		t.Fatalf("identity at its new place: %q %v", moved, readErr)
	}
	if len(run.Signals) != 1 || run.Signals[0] != system.SigTerm || d.started != 1 {
		t.Fatalf("signals %v, restarts %d", run.Signals, d.started)
	}
	if _, statErr := files.Stat("/home/u/.studio"); statErr == nil {
		t.Fatal("~/.studio still there after the triple")
	}
	if got := report.Steps[2]; got.State != catalog.StateOK || got.Detail != "moved to the XDG folders" {
		t.Fatalf("layout step %+v", got)
	}
}

func TestCheckNamesTheFirstRepairAHostNeeds(t *testing.T) {
	run, files := systemtest.NewRunner(t), systemtest.NewFS()
	_ = files.WriteFile("/home/u/.studio/config.json", []byte("{}"), 0o600)

	report, err := build(catalog.Host, false, &registry{}, &daemon{}, run, files).Check(context.Background())

	if err != nil {
		t.Fatal(err)
	}
	want := []struct {
		name  string
		state catalog.State
	}{{"name", catalog.StateAbsent}, {"zshrc", catalog.StateAbsent}, {"layout", catalog.StateDegraded}}
	for i, w := range want {
		if got := report.Steps[i]; got.Name != w.name || got.State != w.state {
			t.Fatalf("step %d: %+v, want %v", i, got, w)
		}
	}
}

func TestTheDefaultNameIsTheLocalHostNameAsAMachineName(t *testing.T) {
	run := systemtest.NewRunner(t)
	run.On("scutil", "--get", "LocalHostName").Fail(system.ErrNotFound)

	got := build(catalog.Host, false, &registry{}, &daemon{}, run, systemtest.NewFS()).DefaultName(context.Background())

	if got != "jean-baptiste-s-macbook-pro" || !setup.ValidName(got) {
		t.Fatalf("default name %q", got)
	}
}
