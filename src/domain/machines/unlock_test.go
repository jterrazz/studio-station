package machines_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jterrazz/studio-station/src/domain/machines"
	"github.com/jterrazz/studio-station/src/system"
	"github.com/jterrazz/studio-station/src/system/systemtest"
)

func precheck() []string {
	return []string{
		"-o", "ConnectTimeout=2", "-o", "BatchMode=yes",
		"-o", "StrictHostKeyChecking=accept-new", "-o", "UserKnownHostsFile=/state/known_hosts/x.preboot",
		"x", "true",
	}
}

func TestUnlockRefusesAHostThatDoesNotAnswer(t *testing.T) {
	run := systemtest.NewRunner(t)
	run.On("ssh", precheck()...).Reply(system.Output{ExitCode: 255, Stderr: "ssh: connect to host x port 22: Operation timed out\n"})
	svc, _ := service(t, run)

	err := svc.Unlock(context.Background(), "x")

	var unreachable *machines.UnreachableError
	if !errors.As(err, &unreachable) || unreachable.Name != "x" {
		t.Fatalf("Unlock error = %v, want UnreachableError for x", err)
	}
}

func TestUnlockOpensTheSessionOnAPreBootHostThatRefusesTheKey(t *testing.T) {
	run := systemtest.NewRunner(t)
	run.On("ssh", precheck()...).Reply(system.Output{ExitCode: 255, Stderr: "x: Permission denied (publickey,password).\n"})
	session := run.On("ssh",
		"-o", "PreferredAuthentications=password", "-o", "PubkeyAuthentication=no",
		"-o", "StrictHostKeyChecking=accept-new", "-o", "UserKnownHostsFile=/state/known_hosts/x.preboot",
		"x").Reply(system.Output{ExitCode: 255})
	svc, _ := service(t, run)

	if err := svc.Unlock(context.Background(), "x"); err != nil {
		t.Fatalf("Unlock: %v", err)
	}
	if session.Seen != 1 {
		t.Fatalf("the session ran %d times, want once, and its exit 255 is not an error", session.Seen)
	}
}
