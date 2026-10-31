package kit

import "github.com/jterrazz/studio-station/src/fail"

func remoteFailure(_ error) (*fail.Error, bool) {
	return nil, false
}
