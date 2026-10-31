package tailnet_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jterrazz/studio-station/src/domain/settings"
	"github.com/jterrazz/studio-station/src/domain/tailnet"
	"github.com/jterrazz/studio-station/src/system"
	"github.com/jterrazz/studio-station/src/system/systemtest"
)

const state = "/state/tailscaled"

// warmingUp answers `tailscale status` as a daemon that needs a few polls to
// come up, as a real one does after it is started.
type warmingUp struct {
	*systemtest.Runner
	failures int
}

func (w *warmingUp) Output(ctx context.Context, c system.Cmd) (system.Output, error) {
	if c.Name == "tailscale" && strings.HasSuffix(strings.Join(c.Args, " "), "status --json") && w.failures > 0 {
		w.failures--
		return system.Output{ExitCode: 1}, nil
	}
	return w.Runner.Output(ctx, c)
}

func fixture(t *testing.T, run system.Runner, runner *systemtest.Runner) (*tailnet.Service, *systemtest.Clock) {
	t.Helper()
	files := systemtest.NewFS()
	clock := &systemtest.Clock{At: time.Unix(0, 0)}
	runner.Paths["tailscale"], runner.Paths["tailscaled"] = "/bin/tailscale", "/bin/tailscaled"
	_ = files.WriteFile(state+"/tailscaled.state", []byte("node"), 0o600)
	h := system.Host{Run: run, FS: files, Env: systemtest.Env{HomeDir: "/home"}, Net: systemtest.NewNet(), Clock: clock}
	paths := settings.Paths{Config: "/config", State: "/state"}
	store := settings.NewStore(files, paths)
	if err := store.Save(settings.Settings{Here: "macbook", Tailnet: settings.Tailnet{Mode: settings.ModeUserspace, AuthMethod: settings.AuthLogin}}); err != nil {
		t.Fatal(err)
	}
	return tailnet.New(h, store, settings.NewKeychain(runner), tailnet.Options{Paths: paths}), clock
}

func TestUpRetriesWithTheFlagsTailscaleSuggests(t *testing.T) {
	runner := systemtest.NewRunner(t)
	socket := "--socket=/state/tailscaled/tailscaled.sock"
	runner.On("tailscale", socket, "status", "--json").Reply(system.Output{Stdout: `{"BackendState":"Running"}`})
	runner.On("tailscale", socket, "up", "--hostname=macbook").Reply(system.Output{ExitCode: 1, Stderr: "Error: changing settings via 'tailscale up' requires mentioning all\nnon-default flags. To proceed, specify --ssh --hostname=old:\n\n\ttailscale up --ssh --hostname=old\n"})
	retried := runner.On("tailscale", socket, "up", "--ssh", "--hostname=macbook").Reply(system.Output{})
	svc, _ := fixture(t, runner, runner)

	result, err := svc.Up(context.Background())

	if err != nil {
		t.Fatalf("Up: %v", err)
	}
	if retried.Seen != 1 || result.Daemon != tailnet.Userspace {
		t.Fatalf("retry seen %d, daemon %q", retried.Seen, result.Daemon)
	}
}

func TestUpNamesTheRefusalWhenTheRetryFailsToo(t *testing.T) {
	runner := systemtest.NewRunner(t)
	socket := "--socket=/state/tailscaled/tailscaled.sock"
	runner.On("tailscale", socket, "status", "--json").Reply(system.Output{Stdout: `{"BackendState":"Running"}`})
	runner.On("tailscale", socket, "up", "--hostname=macbook").Reply(system.Output{ExitCode: 1, Stderr: "Error: requires mentioning all non-default flags\n  tailscale up --ssh\n"})
	runner.On("tailscale", socket, "up", "--ssh", "--hostname=macbook").Reply(system.Output{ExitCode: 1, Stderr: "Error: backend is stopped\n"})
	svc, _ := fixture(t, runner, runner)

	_, err := svc.Up(context.Background())

	var refused *tailnet.AuthError
	if !errors.As(err, &refused) || refused.Detail != "backend is stopped" {
		t.Fatalf("Up error = %v", err)
	}
}

func TestUpWaitsForTheDaemonItJustStarted(t *testing.T) {
	runner := systemtest.NewRunner(t)
	socket := "--socket=/state/tailscaled/tailscaled.sock"
	runner.On("tailscale", socket, "status", "--json").Reply(system.Output{Stdout: `{"BackendState":"Running"}`})
	runner.On("tailscaled", "--tun=userspace-networking", "--state=/state/tailscaled/tailscaled.state", "--socket=/state/tailscaled/tailscaled.sock")
	runner.On("tailscale", socket, "up", "--hostname=macbook").Reply(system.Output{})
	svc, clock := fixture(t, &warmingUp{Runner: runner, failures: 3}, runner)

	if _, err := svc.Up(context.Background()); err != nil {
		t.Fatalf("Up: %v", err)
	}
	if waited := clock.At.Sub(time.Unix(0, 0)); waited != 500*time.Millisecond {
		t.Fatalf("waited %v for the daemon, want 500ms (two polls past the first check)", waited)
	}
}

func TestUpGivesUpOnADaemonThatNeverComes(t *testing.T) {
	runner := systemtest.NewRunner(t)
	runner.On("tailscaled", "--tun=userspace-networking", "--state=/state/tailscaled/tailscaled.state", "--socket=/state/tailscaled/tailscaled.sock")
	svc, clock := fixture(t, &warmingUp{Runner: runner, failures: 1000}, runner)

	_, err := svc.Up(context.Background())

	var down *tailnet.DaemonError
	if !errors.As(err, &down) || !strings.Contains(down.Detail, "did not become ready") {
		t.Fatalf("Up error = %v", err)
	}
	if waited := clock.At.Sub(time.Unix(0, 0)); waited != 4*time.Second {
		t.Fatalf("waited %v, want the 4s budget", waited)
	}
}
