package kit

import (
	"errors"

	"github.com/jterrazz/studio-station/src/core/fail"
	"github.com/jterrazz/studio-station/src/domain/settings"
)

func settingsFailure(err error) (*fail.Error, bool) {
	var unreadable *settings.UnreadableError
	var unwritable *settings.UnwritableError
	var keychain *settings.KeychainError
	switch {
	case errors.As(err, &unreadable):
		return fail.Failed("cannot read \""+unreadable.Path+"\": "+unreadable.Cause.Error(), "repair the JSON of \""+unreadable.Path+"\", or move it away to start afresh", err), true
	case errors.As(err, &unwritable):
		return fail.Failed("cannot write \""+unwritable.Path+"\": "+unwritable.Cause.Error(), "check the permissions of \""+unwritable.Path+"\"", err), true
	case errors.Is(err, settings.ErrNoKeychain):
		return fail.Usage("kept secrets live only in a host's keychain", "run it on the host: station <command> --on <host>"), true
	case errors.As(err, &keychain):
		return fail.Failed("keychain account \""+keychain.Account+"\": "+keychain.Cause.Error(), "open Keychain Access and check the station items", err), true
	}
	return nil, false
}
