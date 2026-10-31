package config

import (
	"context"

	"github.com/jterrazz/studio-station/src/catalog"
)

func (s *Service) checkGitHubLogin(ctx context.Context) (state catalog.State, detail string) {
	if _, ok := s.probe(ctx, "gh", "auth", "status"); ok {
		return catalog.StateOK, "authenticated"
	}
	return catalog.StateAbsent, ""
}

func (s *Service) installGitHubLogin(ctx context.Context) (Change, error) {
	if err := s.attach(ctx, "", "gh", "auth", "login", "--hostname", "github.com", "--git-protocol", "ssh", "--web", "--skip-ssh-key"); err != nil {
		return Change{}, err
	}
	return Change{Steps: []Step{okStep("gh", "logged in to github.com over ssh")}}, nil
}
