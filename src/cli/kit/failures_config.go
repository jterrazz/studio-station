package kit

import (
	"errors"

	"github.com/jterrazz/studio-station/src/domain/config"
	"github.com/jterrazz/studio-station/src/fail"
)

func configFailure(err error) (*fail.Error, bool) {
	var unknown *config.UnknownItemError
	var root *config.NeedsRootError
	var place *config.WrongPlaceError
	var irreversible *config.NotReversibleError
	var terminal *config.NeedsTerminalError
	var studio *config.StudioMissingError
	switch {
	case errors.As(err, &unknown):
		return fail.Usage(unknown.Error(), "run station config for the items"), true
	case errors.As(err, &root):
		return fail.Failed(root.Error(), "sudo station config "+root.Verb+" "+root.Item, err), true
	case errors.As(err, &place):
		return fail.Usage(place.Error(), "run it on a host: station config install "+place.Item+" --on <host>"), true
	case errors.As(err, &irreversible):
		return fail.Usage(irreversible.Error(), "station applies it but cannot undo it: undo it by hand"), true
	case errors.As(err, &terminal):
		return fail.Usage(terminal.Error(), "run it on a terminal"), true
	case errors.As(err, &studio):
		return fail.Failed(studio.Error(), "see docs/reference/secrets.md#claude-in-a-vm", err), true
	}
	return nil, false
}
