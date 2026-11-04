package kit

import (
	"errors"

	"github.com/jterrazz/studio-station/src/core/fail"
	"github.com/jterrazz/studio-station/src/domain/power"
)

func powerFailure(err error) (*fail.Error, bool) {
	var rng *power.RangeError
	var off *power.OffHostError
	var none *power.UnsupportedError
	var guard *power.GuardError
	var down *power.DaemonDownError
	switch {
	case errors.As(err, &rng):
		if rng.Long {
			return usage(rng.Error(), "station power awake --for 24h"), true
		}
		return usage(rng.Error(), "station power awake --for 1h"), true
	case errors.As(err, &off):
		return usage(off.Error(), "ssh <host> station power"), true
	case errors.As(err, &none):
		return usage(none.Error(), "run it on a Mac host"), true
	case errors.As(err, &guard):
		if guard.Guard == power.ReasonLid {
			return failed(guard.Error(), "station power awake --lid --battery", err), true
		}
		return failed(guard.Error(), "station power", err), true
	case errors.As(err, &down):
		if down.Installed {
			return failed(down.Error(), "station engine start", err), true
		}
		return failed(down.Error(), "station setup", err), true
	}
	return nil, false
}
