package config

import (
	"context"
	"encoding/json"
	"errors"
	"slices"

	"github.com/jterrazz/studio-station/src/catalog"
)

const claudeTokenName = "operator:claude:oauth-token"

func (s *Service) checkClaudeToken(ctx context.Context) (state catalog.State, detail string) {
	if _, ok := s.h.Run.LookPath("studio"); !ok {
		return catalog.StateAbsent, ""
	}
	if _, ok := s.probe(ctx, "studio", "credentials", "export", claudeTokenName); ok {
		return catalog.StateOK, "kept by the studio credential store"
	}
	return catalog.StateAbsent, ""
}

// invariant: the token travels on studio's stdin, never in argv, and the OS
// credential store (not station) keeps it.
func (s *Service) installClaudeToken(ctx context.Context, values map[string]string) (Change, error) {
	if _, ok := s.h.Run.LookPath("studio"); !ok {
		return Change{}, &StudioMissingError{Item: "claude-token"}
	}
	payload, err := json.Marshal(map[string]string{claudeTokenName: values["token"]})
	if err != nil {
		return Change{}, err
	}
	if err = s.mustWith(ctx, string(payload), "studio", "credentials", "import"); err != nil {
		return Change{}, errors.New("studio credentials import failed: " + firstLine(err.Error()))
	}
	change := Change{Steps: []Step{okStep(claudeTokenName, "kept by the studio credential store")}}
	current, err := s.store.Load()
	if err != nil {
		return change, err
	}
	if !slices.Contains(current.LendList, claudeTokenName) {
		current.LendList = append(current.LendList, claudeTokenName)
		if err := s.store.Save(current); err != nil {
			return change, err
		}
	}
	change.Steps = append(change.Steps, okStep("lend list", "holds "+claudeTokenName))
	return change, nil
}

func (s *Service) uninstallClaudeToken(ctx context.Context) (Change, error) {
	if _, ok := s.h.Run.LookPath("studio"); !ok {
		return Change{}, &StudioMissingError{Item: "claude-token"}
	}
	if _, err := s.must(ctx, "studio", "credentials", "delete", claudeTokenName); err != nil {
		return Change{}, err
	}
	change := Change{Steps: []Step{okStep(claudeTokenName, "deleted from the studio credential store")}}
	current, err := s.store.Load()
	if err != nil {
		return change, err
	}
	current.LendList = slices.DeleteFunc(current.LendList, func(n string) bool { return n == claudeTokenName })
	if err := s.store.Save(current); err != nil {
		return change, err
	}
	change.Steps = append(change.Steps, okStep("lend list", "no longer holds "+claudeTokenName))
	return change, nil
}
