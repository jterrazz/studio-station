package config

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/system"
)

func TestTailnetAuthKeepsTheKeyInTheKeychainOnlyAndTheChoiceInSettings(t *testing.T) {
	svc, run, files := newTestService(t, catalog.Host, 501)
	run.On("security", "-i").Reply(system.Output{})

	_, err := svc.Install(context.Background(), "tailnet-auth", map[string]string{"mode": "auto", "method": "authkey", "authkey": "tskey-secret-1"}, true)
	if err != nil {
		t.Fatal(err)
	}
	if got := run.Calls[0].Stdin; !strings.Contains(got, `-a "tailnet-authkey" -w "tskey-secret-1"`) {
		t.Fatalf("keychain stdin %q", got)
	}
	for _, c := range run.Calls {
		if strings.Contains(strings.Join(c.Args, " "), "tskey-secret-1") {
			t.Fatalf("the key reached argv: %v", c.Args)
		}
	}
	saved, err := files.ReadFile("/home/u/.config/station/settings.json")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(saved), "tskey-secret-1") || !strings.Contains(string(saved), `"auth_method": "authkey"`) || !strings.Contains(string(saved), `"mode": "auto"`) {
		t.Fatalf("settings.json: %s", saved)
	}
}

func TestTailnetAuthByLoginDropsAKeyAnEarlierChoiceLeft(t *testing.T) {
	svc, run, _ := newTestService(t, catalog.Host, 501)
	run.On("security", "delete-generic-password", "-s", "station", "-a", "tailnet-authkey").Reply(system.Output{})

	if _, err := svc.Install(context.Background(), "tailnet-auth", map[string]string{"mode": "userspace", "method": "login"}, true); err != nil {
		t.Fatal(err)
	}
	if run.Calls[0].Name != "security" || run.Calls[0].Args[0] != "delete-generic-password" {
		t.Fatalf("calls %+v", run.Calls)
	}
}

func TestTailnetAuthRefusesAnAuthKeyLoginWithoutTheKey(t *testing.T) {
	svc, _, _ := newTestService(t, catalog.Host, 501)
	_, err := svc.Install(context.Background(), "tailnet-auth", map[string]string{"mode": "auto", "method": "authkey"}, true)
	if err == nil || !strings.Contains(err.Error(), "auth key") {
		t.Fatalf("got %v", err)
	}
}

func TestVMJoinWritesTheTwoKeychainAccounts(t *testing.T) {
	svc, run, _ := newTestService(t, catalog.Host, 501)
	run.On("security", "-i").Reply(system.Output{})

	if _, err := svc.Install(context.Background(), "vm-join", map[string]string{"client-id": "kid", "client-secret": "ksecret"}, true); err != nil {
		t.Fatal(err)
	}
	if len(run.Calls) != 2 || !strings.Contains(run.Calls[0].Stdin, `-a "vm-join-id" -w "kid"`) || !strings.Contains(run.Calls[1].Stdin, `-a "vm-join-secret" -w "ksecret"`) {
		t.Fatalf("calls %+v", run.Calls)
	}
}

func TestAnInstallMissingARequiredValueFailsBeforeAnyEffect(t *testing.T) {
	svc, run, _ := newTestService(t, catalog.Host, 501)
	change, err := svc.Install(context.Background(), "vm-join", map[string]string{"client-id": "kid"}, true)
	if err == nil || change.Result != catalog.StateBroken || len(run.Calls) != 0 {
		t.Fatalf("err %v, result %q, calls %d", err, change.Result, len(run.Calls))
	}
}

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

func TestClaudeTokenGoesToStudioOnStdinAndOntoTheLendList(t *testing.T) {
	svc, run, files := newTestService(t, catalog.Host, 501)
	run.Paths["studio"] = "/usr/local/bin/studio"
	run.On("studio", "credentials", "import").Reply(system.Output{})

	for range 2 {
		if _, err := svc.Install(context.Background(), "claude-token", claudeValues("sk-ant-oat01-x"), true); err != nil {
			t.Fatal(err)
		}
	}
	var sent map[string]string
	if err := json.Unmarshal([]byte(run.Calls[0].Stdin), &sent); err != nil || sent["operator:claude:oauth-token"] != "sk-ant-oat01-x" || len(sent) != 1 {
		t.Fatalf("stdin %q (%v)", run.Calls[0].Stdin, err)
	}
	if slices.ContainsFunc(run.Calls, func(c system.Cmd) bool { return strings.Contains(strings.Join(c.Args, " "), "sk-ant") }) {
		t.Fatal("the token reached argv")
	}
	saved, _ := files.ReadFile("/home/u/.config/station/settings.json")
	if strings.Count(string(saved), "operator:claude:oauth-token") != 1 || strings.Contains(string(saved), "sk-ant") {
		t.Fatalf("settings.json: %s", saved)
	}
}

func TestClaudeTokenUninstallDeletesItAndTakesItOffTheLendList(t *testing.T) {
	svc, run, files := newTestService(t, catalog.Host, 501)
	run.Paths["studio"] = "/usr/local/bin/studio"
	run.On("studio", "credentials", "import").Reply(system.Output{})
	run.On("studio", "credentials", "delete", "operator:claude:oauth-token").Reply(system.Output{})
	if _, err := svc.Install(context.Background(), "claude-token", claudeValues("t"), true); err != nil {
		t.Fatal(err)
	}

	if _, err := svc.Uninstall(context.Background(), "claude-token"); err != nil {
		t.Fatal(err)
	}
	saved, _ := files.ReadFile("/home/u/.config/station/settings.json")
	if strings.Contains(string(saved), "operator:claude:oauth-token") {
		t.Fatalf("still on the lend list: %s", saved)
	}
}

func TestClaudeTokenNeedsTheStudioCLI(t *testing.T) {
	svc, _, _ := newTestService(t, catalog.Host, 501)
	_, err := svc.Install(context.Background(), "claude-token", claudeValues("t"), true)
	var missing *StudioMissingError
	if !errors.As(err, &missing) {
		t.Fatalf("got %v", err)
	}
}

func TestAnItemWithValuesRefusesOffATerminalBeforeAnyEffect(t *testing.T) {
	svc, run, _ := newTestService(t, catalog.Host, 0)
	for _, name := range []string{"tailnet-auth", "vm-join", "autologin", "claude-token", "git-signing"} {
		_, err := svc.Install(context.Background(), name, nil, false)
		var needs *NeedsTerminalError
		if !errors.As(err, &needs) {
			t.Errorf("%s: got %v", name, err)
		}
	}
	if len(run.Calls) != 0 {
		t.Fatalf("calls before the refusal: %+v", run.Calls)
	}
}

func claudeValues(token string) map[string]string { return map[string]string{"token": token} }
