package kit

import (
	"errors"

	"github.com/jterrazz/studio-station/src/core/fail"
	"github.com/jterrazz/studio-station/src/domain/storage"
)

func storageFailure(err error) (*fail.Error, bool) {
	var unknown *storage.UnknownToolError
	var place *storage.WrongPlaceError
	var noClean *storage.NoCleanError
	var noReset *storage.NoResetError
	var noName *storage.NoNameError
	switch {
	case errors.As(err, &unknown):
		return usage(unknown.Error(), "station storage"), true
	case errors.As(err, &place):
		return usage(place.Error(), "station storage"), true
	case errors.As(err, &noClean):
		return usage(noClean.Error(), "station storage"), true
	case errors.As(err, &noReset):
		return usage(noReset.Error(), noReset.Hint), true
	case errors.As(err, &noName):
		return usage(noName.Error(), "station storage reset <tool...>"), true
	}
	return nil, false
}
