package config

import (
	"context"
	"errors"
	"slices"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/domain/settings"
)

const authKeyAccount = "tailnet-authkey"

func (s *Service) checkTailnetAuth(ctx context.Context) (state catalog.State, detail string) {
	current, err := s.store.Load()
	if err != nil {
		return catalog.StateBroken, firstLine(err.Error())
	}
	choice := string(current.Tailnet.Mode) + " · " + string(current.Tailnet.AuthMethod)
	if current.Tailnet.AuthMethod != settings.AuthAuthKey {
		return catalog.StateOK, choice
	}
	found, err := s.kept(ctx, authKeyAccount)
	switch {
	case err != nil:
		return catalog.StateUnknown, firstLine(err.Error())
	case !found:
		return catalog.StateBroken, "the auth key is missing from the keychain"
	}
	return catalog.StateOK, choice
}

// invariant: the auth key goes to the keychain and nowhere else; settings.json
// records the choice (mode, method), never the key.
func (s *Service) installTailnetAuth(ctx context.Context, values map[string]string) (Change, error) {
	mode, method := settings.Mode(values["mode"]), settings.AuthMethod(values["method"])
	if !slices.Contains([]settings.Mode{settings.ModeAuto, settings.ModeUserspace}, mode) {
		return Change{}, errors.New("mode is auto or userspace")
	}
	if !slices.Contains([]settings.AuthMethod{settings.AuthLogin, settings.AuthAuthKey}, method) {
		return Change{}, errors.New("login is login or authkey")
	}
	var change Change
	switch {
	case method == settings.AuthAuthKey && values["authkey"] == "":
		return Change{}, errors.New("an authkey login needs the auth key")
	case method == settings.AuthAuthKey:
		if err := s.kc.Write(ctx, authKeyAccount, values["authkey"]); err != nil {
			return change, err
		}
		change.Steps = append(change.Steps, okStep("auth key", "kept in the keychain"))
	default:
		if err := s.kc.Remove(ctx, authKeyAccount); err != nil {
			return change, err
		}
	}
	current, err := s.store.Load()
	if err != nil {
		return change, err
	}
	current.Tailnet = settings.Tailnet{Mode: mode, AuthMethod: method}
	if err := s.store.Save(current); err != nil {
		return change, err
	}
	change.Steps = append(change.Steps, okStep("settings", string(mode)+" · "+string(method)))
	return change, nil
}

func (s *Service) uninstallTailnetAuth(ctx context.Context) (Change, error) {
	if err := s.kc.Remove(ctx, authKeyAccount); err != nil {
		return Change{}, err
	}
	current, err := s.store.Load()
	if err != nil {
		return Change{}, err
	}
	current.Tailnet = settings.Defaults().Tailnet
	if err := s.store.Save(current); err != nil {
		return Change{}, err
	}
	return Change{Steps: []Step{okStep("auth key", "removed from the keychain"), okStep("settings", "back to the defaults")}}, nil
}
