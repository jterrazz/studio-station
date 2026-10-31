package config

import (
	"context"
	"errors"
	"io/fs"

	"github.com/jterrazz/studio-station/src/catalog"
)

const (
	agentUser     = "jterrazz.agent"
	agentHome     = "/Users/" + agentUser
	loginWindow   = "/Library/Preferences/com.apple.loginwindow"
	kcpasswordTo  = "/etc/kcpassword"
	autologinHint = "verify end to end: sudo fdesetup authrestart -delayminutes 0 (or station machines restart <name>)"
)

func (s *Service) installAutologin(ctx context.Context, values map[string]string) (Change, error) {
	if err := s.requireDarwin(); err != nil {
		return Change{}, err
	}
	if _, ok := s.h.Run.LookPath("sysadminctl"); !ok {
		return Change{}, errors.New("sysadminctl not found")
	}
	if !s.exists(agentHome) {
		return Change{}, errors.New("user " + agentUser + " does not exist on this Mac")
	}
	password := values["password"]
	if password == "" {
		return Change{}, errors.New("empty password: refusing to set " + kcpasswordTo + " with an empty value")
	}
	change := Change{Before: s.autologinFacts(ctx)}
	if _, err := s.must(ctx, "defaults", "write", loginWindow, "DisableFDEAutoLogin", "-bool", "NO"); err != nil {
		return change, err
	}
	change.Steps = append(change.Steps, okStep("DisableFDEAutoLogin = NO", ""))

	// outside: on macOS 26 `sysadminctl -autoLogin set` refuses on a FileVault disk
	// whatever DisableFDEAutoLogin says, so the login is written as /etc/kcpassword (the
	// XOR cipher loginwindow has read since 10.4) plus autoLoginUser; loginwindow at boot
	// still honors DisableFDEAutoLogin.
	if err := s.h.FS.WriteFile(kcpasswordTo, encodeKCPassword(password), 0o600); err != nil {
		return change, errors.New("write " + kcpasswordTo + ": " + err.Error())
	}
	if err := s.h.FS.Chmod(kcpasswordTo, 0o600); err != nil {
		return change, err
	}
	// outside: /etc/kcpassword must be owned by root:wheel.
	_ = s.h.FS.Chown(kcpasswordTo, 0, 0)
	change.Steps = append(change.Steps, okStep(kcpasswordTo, "written (root:wheel 0600)"))

	if _, err := s.must(ctx, "defaults", "write", loginWindow, "autoLoginUser", "-string", agentUser); err != nil {
		return change, err
	}
	change.Steps = append(change.Steps, okStep("autoLoginUser = "+agentUser, ""))
	if !s.exists(kcpasswordTo) {
		return change, errors.New(kcpasswordTo + " missing after write")
	}
	if got, ok := s.probe(ctx, "defaults", "read", loginWindow, "autoLoginUser"); !ok || got != agentUser {
		return change, errors.New("autoLoginUser cross-check failed (got \"" + got + "\")")
	}
	// value: clearing the screen-locked-after-resume flag keeps the auto-login session
	// interactive; lock-after-login owns the lock itself.
	_, _ = s.probe(ctx, "defaults", "delete", loginWindow, "autoLoginUserScreenLocked")

	change.After = s.autologinFacts(ctx)
	change.Steps = append(change.Steps, skipStep("next", autologinHint))
	return change, nil
}

func (s *Service) uninstallAutologin(ctx context.Context) (Change, error) {
	if err := s.requireDarwin(); err != nil {
		return Change{}, err
	}
	_, _ = s.probe(ctx, "sysadminctl", "-autologin", "off")
	if err := s.h.FS.Remove(kcpasswordTo); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return Change{}, err
	}
	for _, key := range []string{"autoLoginUser", "oneTimeAutoLogin", "autoLoginUserScreenLocked"} {
		_, _ = s.probe(ctx, "defaults", "delete", loginWindow, key)
	}
	return Change{
		Steps: []Step{okStep(kcpasswordTo, "removed"), okStep("autoLoginUser", "cleared")},
		After: s.autologinFacts(ctx),
	}, nil
}

func (s *Service) checkAutologin(ctx context.Context) (state catalog.State, detail string) {
	if got, ok := s.probe(ctx, "defaults", "read", loginWindow, "autoLoginUser"); !ok || got != agentUser {
		return catalog.StateAbsent, ""
	}
	if !s.exists(kcpasswordTo) {
		return catalog.StateDegraded, kcpasswordTo + " is missing"
	}
	return catalog.StateOK, agentUser
}

func (s *Service) autologinFacts(ctx context.Context) []Fact {
	var facts []Fact
	for _, key := range []string{"autoLoginUser", "DisableFDEAutoLogin"} {
		value := "(unset)"
		if got, ok := s.probe(ctx, "defaults", "read", loginWindow, key); ok {
			value = oneLineOrDash(got)
		}
		facts = append(facts, Fact{Key: key, Value: value})
	}
	kc := "absent"
	if s.exists(kcpasswordTo) {
		kc = "present"
	}
	return append(facts, Fact{Key: kcpasswordTo, Value: kc})
}
