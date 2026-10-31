package config

import (
	"context"
	"strings"
	"testing"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/system"
)

func TestGitSigningWiresTheKeyGpgListsIntoGit(t *testing.T) {
	svc, run, _ := newTestService(t, catalog.Host, 501)
	run.Paths["gpg"] = "/opt/homebrew/bin/gpg"
	listing := "sec   ed25519/3AA5C34371567BD2 2026-10-31 [SC]\n      FINGERPRINT\nuid           [ultimate] Jean-Baptiste <admin@jterrazz.com>\n"
	run.On("gpg", "--list-secret-keys", "--keyid-format", "long", "admin@jterrazz.com").Reply(system.Output{Stdout: listing})
	signing := run.On("git", "config", "--global", "user.signingkey", "3AA5C34371567BD2").Reply(system.Output{})
	run.On("git", "config", "--global", "commit.gpgsign", "true").Reply(system.Output{})
	run.On("git", "config", "--global", "gpg.program", "gpg").Reply(system.Output{})
	run.On("gpg", "--armor", "--export", "admin@jterrazz.com").Reply(system.Output{})

	if _, err := svc.Install(context.Background(), "git-signing", nil, true); err != nil {
		t.Fatal(err)
	}
	if signing.Seen != 1 {
		t.Fatalf("git was told the signing key %d times", signing.Seen)
	}
}

func TestSSHKeyAppendsTheKeychainStanzaOnceAndKeepsWhatWasThere(t *testing.T) {
	svc, run, files := newTestService(t, catalog.Host, 501)
	run.On("ssh-add", "--apple-use-keychain", "/home/u/.ssh/id_ed25519").Reply(system.Output{})
	mustWrite(t, files.WriteFile("/home/u/.ssh/id_ed25519", []byte("private"), 0o600))
	mustWrite(t, files.WriteFile("/home/u/.ssh/id_ed25519.pub", []byte("ssh-ed25519 AAAA u\n"), 0o644))
	mustWrite(t, files.WriteFile("/home/u/.ssh/config", []byte("Host work\n  User w\n"), 0o600))

	for range 2 {
		change, err := svc.Install(context.Background(), "ssh-key", nil, true)
		if err != nil {
			t.Fatal(err)
		}
		if len(change.After) != 1 || change.After[0].Value != "ssh-ed25519 AAAA u" {
			t.Fatalf("public key fact %+v", change.After)
		}
	}
	got, _ := files.ReadFile("/home/u/.ssh/config")
	if !strings.HasPrefix(string(got), "Host work\n  User w\n") || strings.Count(string(got), "AddKeysToAgent yes") != 1 {
		t.Fatalf("~/.ssh/config:\n%s", got)
	}
}

func mustWrite(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
