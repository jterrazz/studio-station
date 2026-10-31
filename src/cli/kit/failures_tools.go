package kit

import "github.com/jterrazz/studio-station/src/fail"

func toolsFailure(_ error) (*fail.Error, bool) {
	return nil, false
}
