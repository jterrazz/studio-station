package kit

import (
	"errors"

	"github.com/jterrazz/studio-station/src/core/fail"
	"github.com/jterrazz/studio-station/src/domain/shortcuts"
)

func shortcutsFailure(err error) (*fail.Error, bool) {
	var git *shortcuts.GitStepError
	var docker *shortcuts.DockerStepError
	var missing *shortcuts.NotInstalledError
	var unknown *shortcuts.UnknownShortcutError
	switch {
	case errors.As(err, &git):
		return fail.Failed(git.Error(), "fix what git printed above, then run station run git "+git.Shortcut+" again", err), true
	case errors.As(err, &docker):
		return fail.Failed(docker.Error(), "check that the docker daemon runs, then run it again", err), true
	case errors.As(err, &missing):
		return fail.Failed(missing.Error(), "station tools install "+missing.Tool, err), true
	case errors.As(err, &unknown):
		return fail.Usage(unknown.Error(), "run station run --help"), true
	}
	return nil, false
}
