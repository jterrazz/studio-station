package config

import (
	"context"
	"strings"
	"testing"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/system"
)

func TestSshdInstallFailsWhenSystemsetupFails(t *testing.T) {
	svc, run, _ := newTestService(t, catalog.Host, 0)
	run.On("systemsetup", "-getremotelogin").Reply(system.Output{Stdout: "Remote Login: Off\n"})
	run.On("systemsetup", "-setremotelogin", "on").Reply(system.Output{ExitCode: 1, Stderr: "setremotelogin: Turning Remote Login on or off requires Full Disk Access privileges.\n"})

	change, err := svc.Install(context.Background(), "sshd", nil, true)
	if err == nil || !strings.Contains(err.Error(), "Full Disk Access") {
		t.Fatalf("a failing systemsetup read as success: %v", err)
	}
	if change.Result != catalog.StateBroken {
		t.Fatalf("result %q for a failed install", change.Result)
	}
}
