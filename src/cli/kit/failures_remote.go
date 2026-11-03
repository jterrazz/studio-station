package kit

import (
	"errors"

	"github.com/jterrazz/studio-station/src/core/fail"
	"github.com/jterrazz/studio-station/src/domain/remote"
)

func remoteFailure(err error) (*fail.Error, bool) {
	var unreachable *remote.UnreachableError
	if errors.As(err, &unreachable) {
		return failed(unreachable.Error(), "station machines probe "+unreachable.Machine, err), true
	}
	return nil, false
}
