package config

import (
	"context"

	"github.com/jterrazz/studio-station/src/catalog"
	settingscontract "github.com/jterrazz/studio-station/src/settings/contract"
)

// invariant: an item runs through the Applier of its effect's kind; the one
// value switch over Procedure has no default case, so the exhaustive linter
// names every Procedure a new constant forgets, and dispatch_test.go walks
// the catalogue.
func (s *Service) handler(it catalog.ConfigItem) (handler, error) {
	a, ok := s.appliers.For(it)
	if !ok {
		return handler{}, &unhandledError{Item: it.Name}
	}
	h := handler{
		install: func(ctx context.Context, values map[string]string) (Change, error) { return a.Apply(ctx, it, values) },
	}
	if !it.RunOnce {
		h.check = func(ctx context.Context) (catalog.State, string) {
			o, err := a.Observe(ctx, it)
			if err != nil {
				return catalog.StateBroken, firstLine(err.Error())
			}
			return o.State, o.Detail
		}
	}
	if it.Reversible {
		h.uninstall = func(ctx context.Context) (Change, error) { return a.Undo(ctx, it) }
	}
	return h, nil
}

func (s *Service) procedure(p catalog.Procedure) (handler, bool) {
	switch p {
	case catalog.ProcGitSigning:
		return handler{check: s.checkGitSigning, install: bare(s.installGitSigning)}, true
	case catalog.ProcSSHKey:
		return handler{check: s.checkSSHKey, install: bare(s.installSSHKey)}, true
	case catalog.ProcGitHubLogin:
		return handler{check: s.checkGitHubLogin, install: bare(s.installGitHubLogin)}, true
	case catalog.ProcSpotlight:
		return handler{check: s.checkSpotlight, install: bare(s.installSpotlight)}, true
	case catalog.ProcDNS:
		return handler{check: s.checkDNS, install: bare(s.installDNS)}, true
	case catalog.ProcDockReset:
		return handler{install: bare(s.installDockReset)}, true
	case catalog.ProcDockSpacer:
		return handler{install: bare(s.installDockSpacer)}, true
	case catalog.ProcAutologin:
		return handler{check: s.checkAutologin, install: s.installAutologin, uninstall: s.uninstallAutologin}, true
	case catalog.ProcPower:
		return handler{check: s.checkPower, install: bare(s.installPower), uninstall: s.uninstallPower}, true
	case catalog.ProcLockAfterLogin:
		return handler{check: s.checkLockAfterLogin, install: bare(s.installLockAfterLogin), uninstall: s.uninstallLockAfterLogin}, true
	case catalog.ProcSshd:
		return handler{check: s.checkSshd, install: bare(s.installSshd), uninstall: s.uninstallSshd}, true
	case catalog.ProcClaudeToken:
		return handler{check: s.checkClaudeToken, install: s.installClaudeToken, uninstall: s.uninstallClaudeToken}, true
	case catalog.ProcBackupExclusions:
		return handler{check: s.checkBackupExclusions, install: bare(s.installBackupExclusions), uninstall: s.uninstallBackupExclusions}, true
	}
	return handler{}, false
}

// value: every procedure of the catalogue as the Applier of its own kind.
func (s *Service) procedures() []settingscontract.Applier {
	var out []settingscontract.Applier
	seen := map[catalog.Procedure]bool{}
	for _, it := range s.items {
		p, ok := it.Effect.(catalog.Procedure)
		if !ok || seen[p] {
			continue
		}
		if h, ok := s.procedure(p); ok {
			seen[p] = true
			out = append(out, procedureApplier{p: p, h: h})
		}
	}
	return out
}

func bare(fn func(ctx context.Context) (Change, error)) func(context.Context, map[string]string) (Change, error) {
	return func(ctx context.Context, _ map[string]string) (Change, error) { return fn(ctx) }
}
