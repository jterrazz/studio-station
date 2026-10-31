package config

import "context"

// outside: `defaults delete` fails when the domain is already absent, which is the
// state the reset asks for.
func (s *Service) installDockReset(ctx context.Context) (Change, error) {
	_, _ = s.probe(ctx, "defaults", "delete", "com.apple.dock")
	if _, err := s.must(ctx, "killall", "Dock"); err != nil {
		return Change{}, err
	}
	return Change{Steps: []Step{okStep("dock", "reset to the system defaults")}}, nil
}

func (s *Service) installDockSpacer(ctx context.Context) (Change, error) {
	if _, err := s.must(ctx, "defaults", "write", "com.apple.dock", "persistent-apps", "-array-add", `{"tile-type"="small-spacer-tile";}`); err != nil {
		return Change{}, err
	}
	if _, err := s.must(ctx, "killall", "Dock"); err != nil {
		return Change{}, err
	}
	return Change{Steps: []Step{okStep("dock", "spacer tile added")}}, nil
}
