package kit

import (
	"errors"

	"github.com/jterrazz/studio-station/src/core/fail"
	"github.com/jterrazz/studio-station/src/domain/config"
	"github.com/jterrazz/studio-station/src/surface/cli/words"
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
		return usage(unknown.Error(), "station settings"), true
	case errors.As(err, &root):
		verb := "apply"
		if root.Verb == "uninstall" {
			verb = "off"
		}
		return failed(root.Error(), "sudo station settings "+verb+" "+root.Item, err), true
	case errors.As(err, &place):
		return usage(place.Error(), "station settings apply "+place.Item+" --on this-mac"), true
	case errors.As(err, &irreversible):
		return usage(irreversible.Error(), "station applies it but cannot undo it: undo it by hand"), true
	case errors.As(err, &terminal):
		return Translate(fail.New(fail.NeedsTerminal, words.NeedsTerminal, terminal.Item, nil), ""), true
	case errors.As(err, &studio):
		return failed(studio.Error(), "see docs/reference/secrets.md#claude-in-a-vm", err), true
	}
	return nil, false
}
