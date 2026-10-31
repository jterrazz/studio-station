package kit

import (
	"errors"

	"github.com/jterrazz/studio-station/src/domain/remote"
	"github.com/jterrazz/studio-station/src/fail"
)

func remoteFailure(err error) (*fail.Error, bool) {
	var unreachable *remote.UnreachableError
	if errors.As(err, &unreachable) {
		return fail.Failed(unreachable.Error(), "station machines probe "+unreachable.Machine, err), true
	}
	return nil, false
}
