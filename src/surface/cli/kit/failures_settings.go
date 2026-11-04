package kit

import (
	"errors"

	"github.com/jterrazz/studio-station/src/accounts"
	"github.com/jterrazz/studio-station/src/core/fail"
	"github.com/jterrazz/studio-station/src/core/store"
)

func settingsFailure(err error) (*fail.Error, bool) {
	var unreadable *store.UnreadableError
	var unwritable *store.UnwritableError
	var vault *accounts.VaultError
	switch {
	case errors.As(err, &unreadable):
		return failed("cannot read \""+unreadable.Path+"\": "+unreadable.Cause.Error(), "repair the JSON of \""+unreadable.Path+"\", or move it away to start afresh", err), true
	case errors.As(err, &unwritable):
		return failed("cannot write \""+unwritable.Path+"\": "+unwritable.Cause.Error(), "check the permissions of \""+unwritable.Path+"\"", err), true
	case errors.Is(err, accounts.ErrNoVault):
		return usage(accounts.ErrNoVault.Error(), "run it on the host: station <command> --on <host>"), true
	case errors.As(err, &vault):
		return failed("keychain account \""+vault.Name+"\": "+vault.Cause.Error(), "open Keychain Access and check the station items", err), true
	}
	return nil, false
}
