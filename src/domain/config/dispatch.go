package config

import (
	"context"

	"github.com/jterrazz/studio-station/src/catalog"
)

// invariant: the one type switch over Effect and the one value switch over
// Procedure, the latter with no default case: the exhaustive linter then names every
// Procedure a new constant forgets, and dispatch_test.go walks the catalogue.
func (s *Service) handler(it catalog.ConfigItem) (handler, error) {
	switch e := it.Effect.(type) {
	case catalog.Touch:
		return s.touch(e), nil
	case catalog.Dotfiles:
		return s.dotfiles(e), nil
	case catalog.Procedure:
		return s.procedure(it, e)
	}
	return handler{}, &unhandledError{Item: it.Name}
}

func (s *Service) procedure(it catalog.ConfigItem, p catalog.Procedure) (handler, error) {
	switch p {
	case catalog.ProcGitSigning:
		return handler{check: s.checkGitSigning, install: bare(s.installGitSigning)}, nil
	case catalog.ProcSSHKey:
		return handler{check: s.checkSSHKey, install: bare(s.installSSHKey)}, nil
	case catalog.ProcGitHubLogin:
		return handler{check: s.checkGitHubLogin, install: bare(s.installGitHubLogin)}, nil
	case catalog.ProcSpotlight:
		return handler{check: s.checkSpotlight, install: bare(s.installSpotlight)}, nil
	case catalog.ProcDNS:
		return handler{check: s.checkDNS, install: bare(s.installDNS)}, nil
	case catalog.ProcDockReset:
		return handler{install: bare(s.installDockReset)}, nil
	case catalog.ProcDockSpacer:
		return handler{install: bare(s.installDockSpacer)}, nil
	case catalog.ProcAutologin:
		return handler{check: s.checkAutologin, install: s.installAutologin, uninstall: s.uninstallAutologin}, nil
	case catalog.ProcPower:
		return handler{check: s.checkPower, install: bare(s.installPower), uninstall: s.uninstallPower}, nil
	case catalog.ProcLockAfterLogin:
		return handler{check: s.checkLockAfterLogin, install: bare(s.installLockAfterLogin), uninstall: s.uninstallLockAfterLogin}, nil
	case catalog.ProcSshd:
		return handler{check: s.checkSshd, install: bare(s.installSshd), uninstall: s.uninstallSshd}, nil
	case catalog.ProcClaudeToken:
		return handler{check: s.checkClaudeToken, install: s.installClaudeToken, uninstall: s.uninstallClaudeToken}, nil
	case catalog.ProcBackupExclusions:
		return handler{check: s.checkBackupExclusions, install: bare(s.installBackupExclusions), uninstall: s.uninstallBackupExclusions}, nil
	}
	return handler{}, &unhandledError{Item: it.Name}
}

func bare(fn func(ctx context.Context) (Change, error)) func(context.Context, map[string]string) (Change, error) {
	return func(ctx context.Context, _ map[string]string) (Change, error) { return fn(ctx) }
}
