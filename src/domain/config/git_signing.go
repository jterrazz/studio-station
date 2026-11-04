package config

import (
	"context"
	"errors"
	"strings"

	"github.com/jterrazz/studio-station/src/catalog"
)

func (s *Service) checkGitSigning(ctx context.Context) (state catalog.State, detail string) {
	if out, ok := s.probe(ctx, "git", "config", "--global", "commit.gpgsign"); ok && out == "true" {
		return catalog.StateOK, "commit.gpgsign=true"
	}
	return catalog.StateAbsent, ""
}

func (s *Service) installGitSigning(ctx context.Context) (Change, error) {
	person, err := s.prefs.Identity.Get()
	if err != nil {
		return Change{}, err
	}
	email := person.Email()
	if _, ok := s.h.Run.LookPath("gpg"); !ok {
		return Change{}, errors.New("gpg is not installed: station tools install gpg")
	}
	var change Change
	if out, ok := s.probe(ctx, "gpg", "--list-secret-keys", "--keyid-format", "long", email); ok && out != "" {
		change.Steps = append(change.Steps, okStep("gpg key", "already exists for "+email))
	} else {
		batch := "%no-protection\nKey-Type: eddsa\nKey-Curve: ed25519\nName-Real: " + person.Name() + "\nName-Email: " + email + "\nExpire-Date: 0\n%commit\n"
		if fault := s.attach(ctx, batch, "gpg", "--batch", "--generate-key"); fault != nil {
			return change, fault
		}
		change.Steps = append(change.Steps, okStep("gpg key", "generated for "+email+" (ed25519)"))
	}
	listing, err := s.must(ctx, "gpg", "--list-secret-keys", "--keyid-format", "long", email)
	if err != nil {
		return change, err
	}
	keyID := signingKeyID(listing)
	if keyID == "" {
		return change, errors.New("could not find the gpg key id for " + email)
	}
	for _, kv := range [][2]string{{"user.signingkey", keyID}, {"commit.gpgsign", "true"}, {"gpg.program", "gpg"}} {
		if _, err := s.must(ctx, "git", "config", "--global", kv[0], kv[1]); err != nil {
			return change, err
		}
	}
	change.Steps = append(change.Steps, okStep("git", "signs every commit with "+keyID))
	if err := s.attach(ctx, "", "gpg", "--armor", "--export", email); err != nil {
		return change, err
	}
	change.Steps = append(change.Steps, skipStep("public key", "printed above: add it at https://github.com/settings/gpg/new"))
	return change, nil
}

func signingKeyID(listing string) string {
	for _, line := range strings.Split(listing, "\n") {
		if !strings.Contains(line, "ed25519/") && !strings.Contains(line, "rsa") {
			continue
		}
		if _, after, ok := strings.Cut(line, "/"); ok {
			if fields := strings.Fields(after); len(fields) > 0 {
				return fields[0]
			}
		}
	}
	return ""
}
