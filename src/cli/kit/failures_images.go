package kit

import "github.com/jterrazz/studio-station/src/fail"

func imagesFailure(_ error) (*fail.Error, bool) {
	return nil, false
}
