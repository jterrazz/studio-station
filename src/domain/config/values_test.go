package config

import (
	"bytes"
	"context"
	"testing"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/system"
)

func TestAutologinWritesTheEncodedPasswordFromTheFormOrTheDefault(t *testing.T) {
	svc, run, files := newTestService(t, catalog.Host, 0)
	svc.o.Defaults = map[string]map[string]string{"autologin": {"password": "from-env"}}
	run.Paths["sysadminctl"] = "/usr/sbin/sysadminctl"
	if err := files.MkdirAll("/Users/jterrazz.agent", 0o755); err != nil {
		t.Fatal(err)
	}
	const pane = "/Library/Preferences/com.apple.loginwindow"
	run.On("defaults", "read", pane, "autoLoginUser").Reply(system.Output{Stdout: "jterrazz.agent\n"})
	run.On("defaults", "read", pane, "DisableFDEAutoLogin").Reply(system.Output{Stdout: "0\n"})
	run.On("defaults", "write", pane, "DisableFDEAutoLogin", "-bool", "NO").Reply(system.Output{})
	run.On("defaults", "write", pane, "autoLoginUser", "-string", "jterrazz.agent").Reply(system.Output{})
	run.On("defaults", "delete", pane, "autoLoginUserScreenLocked").Reply(system.Output{})

	if _, err := svc.Install(context.Background(), "autologin", nil, true); err != nil {
		t.Fatal(err)
	}
	got, err := files.ReadFile("/etc/kcpassword")
	if err != nil || !bytes.Equal(got, encodeKCPassword("from-env")) {
		t.Fatalf("kcpassword %x (%v)", got, err)
	}

	if _, err := svc.Install(context.Background(), "autologin", map[string]string{"password": "typed"}, true); err != nil {
		t.Fatal(err)
	}
	got, _ = files.ReadFile("/etc/kcpassword")
	if !bytes.Equal(got, encodeKCPassword("typed")) {
		t.Fatalf("the form's value did not win over the default: %x", got)
	}
}
