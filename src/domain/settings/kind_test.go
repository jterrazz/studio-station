package settings_test

import (
	"context"
	"testing"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/domain/settings"
	"github.com/jterrazz/studio-station/src/system"
	"github.com/jterrazz/studio-station/src/system/systemtest"
)

func kindOn(t *testing.T, script func(*systemtest.Runner)) catalog.MachineKind {
	run := systemtest.NewRunner(t)
	script(run)
	h := systemtest.Host(run, systemtest.NewFS(), systemtest.Env{OS: "darwin"}, systemtest.NewNet(), &systemtest.Clock{})
	return settings.DetectKind(context.Background(), h, "")
}

func TestDetectKindAFailingSysctlIsAHost(t *testing.T) {
	got := kindOn(t, func(r *systemtest.Runner) {
		r.On("sysctl", "-n", "kern.hv_vmm_present").Fail(system.ErrNotFound)
	})
	if got != catalog.Host {
		t.Fatalf("kind %s", got)
	}
}

func TestDetectKindAMacOSGuestSaysSo(t *testing.T) {
	got := kindOn(t, func(r *systemtest.Runner) {
		r.On("sysctl", "-n", "kern.hv_vmm_present").Reply(system.Output{Stdout: "1\n"})
	})
	if got != catalog.MacOS {
		t.Fatalf("kind %s", got)
	}
}
