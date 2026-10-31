package status

import (
	"context"
	"path"
	"strings"

	"github.com/jterrazz/studio-station/src/catalog"
)

func (s *Service) identity(ctx context.Context) []Check {
	want, _ := s.deps.Store.Load()
	table := []probe{
		{name: "git-email", cmd: cmd("git", "config", "--global", "user.email"), interpret: gitValue(want.Email())},
		{name: "git-name", cmd: cmd("git", "config", "--global", "user.name"), interpret: gitValue(want.Name())},
		{name: "git-signing", cmd: cmd("git", "config", "--global", "commit.gpgsign"), interpret: gitSigning},
		{name: "gpg-key", cmd: cmd("gpg", "--list-secret-keys", "--keyid-format", "long"), interpret: gpgKey},
		{name: "github", cmd: cmd("gh", "auth", "status"), interpret: githubLogin},
	}
	checks := s.probes(ctx, table)
	return append(checks, s.sshKey())
}

func (s *Service) sshKey() Check {
	if _, err := s.h.FS.Stat(path.Join(s.h.Env.Home(), ".ssh", "id_ed25519")); err != nil {
		return Check{Name: "ssh-key", State: catalog.StateAbsent}
	}
	return Check{Name: "ssh-key", State: catalog.StateOK, Value: "present", Detail: "~/.ssh/id_ed25519"}
}

// invariant: a global git value that is set but is not the one settings.json expects
// is degraded, not ok: commits would carry the wrong author.
func gitValue(expected string) func(string, reply) Check {
	return func(name string, r reply) Check {
		got := strings.TrimSpace(r.text())
		switch {
		case !r.ok() || got == "":
			return Check{Name: name, State: catalog.StateAbsent}
		case got != expected:
			return Check{Name: name, State: catalog.StateDegraded, Value: got, Detail: "expected " + expected}
		}
		return Check{Name: name, State: catalog.StateOK, Value: got}
	}
}

func gitSigning(name string, r reply) Check {
	if r.ok() && strings.TrimSpace(r.text()) == "true" {
		return Check{Name: name, State: catalog.StateOK, Value: "on"}
	}
	return Check{Name: name, State: catalog.StateAbsent, Value: "off"}
}

func gpgKey(name string, r reply) Check {
	if !r.ok() || strings.TrimSpace(r.text()) == "" {
		return Check{Name: name, State: catalog.StateAbsent}
	}
	return Check{Name: name, State: catalog.StateOK, Value: "present", Detail: "~/.gnupg"}
}

// outside: gh prints `Logged in to github.com account <user> (keyring)`, on stderr
// or stdout by version.
func githubLogin(name string, r reply) Check {
	if !r.ok() {
		return Check{Name: name, State: catalog.StateAbsent}
	}
	_, rest, found := strings.Cut(r.all(), "account ")
	account, _, _ := strings.Cut(rest, " ")
	if !found || account == "" {
		return Check{Name: name, State: catalog.StateOK, Value: "logged in"}
	}
	return Check{Name: name, State: catalog.StateOK, Value: account}
}
