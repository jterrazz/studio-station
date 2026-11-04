package config

import (
	"context"
	"errors"
	"io/fs"
	"strings"

	"github.com/jterrazz/studio-station/src/catalog"
)

const sshHostStanza = "\nHost *\n  AddKeysToAgent yes\n  UseKeychain yes\n  IdentityFile ~/.ssh/id_ed25519\n"

func (s *Service) checkSSHKey(context.Context) (state catalog.State, detail string) {
	if s.exists(s.home("~/.ssh/id_ed25519")) {
		return catalog.StateOK, "~/.ssh/id_ed25519"
	}
	return catalog.StateAbsent, ""
}

func (s *Service) installSSHKey(ctx context.Context) (Change, error) {
	person, err := s.prefs.Identity.Get()
	if err != nil {
		return Change{}, err
	}
	dir, key := s.home("~/.ssh"), s.home("~/.ssh/id_ed25519")
	if fault := s.h.FS.MkdirAll(dir, 0o700); fault != nil {
		return Change{}, fault
	}
	var change Change
	if s.exists(key) {
		change.Steps = append(change.Steps, okStep("ssh key", "already exists"))
	} else {
		if fault := s.attach(ctx, "", "ssh-keygen", "-t", "ed25519", "-C", person.Email(), "-f", key); fault != nil {
			return change, fault
		}
		change.Steps = append(change.Steps, okStep("ssh key", "generated"))
	}
	config := dir + "/config"
	existing, err := s.h.FS.ReadFile(config)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return change, err
	}
	if strings.Contains(string(existing), "AddKeysToAgent yes") {
		change.Steps = append(change.Steps, okStep("~/.ssh/config", "already configured"))
	} else {
		if fault := s.h.FS.WriteFile(config, append(existing, sshHostStanza...), 0o600); fault != nil {
			return change, fault
		}
		change.Steps = append(change.Steps, okStep("~/.ssh/config", "keychain stanza added"))
	}
	if fault := s.attach(ctx, "", "ssh-add", "--apple-use-keychain", key); fault != nil {
		return change, fault
	}
	change.Steps = append(change.Steps, okStep("ssh-add", "passphrase kept in the Keychain"))
	pub, err := s.h.FS.ReadFile(key + ".pub")
	if err != nil {
		return change, err
	}
	change.After = []Fact{{Key: "public key", Value: strings.TrimSpace(string(pub))}}
	change.Steps = append(change.Steps, skipStep("next", "add the public key at https://github.com/settings/ssh/new"))
	return change, nil
}
