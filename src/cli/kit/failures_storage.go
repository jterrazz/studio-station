package kit

import (
	"errors"

	"github.com/jterrazz/studio-station/src/domain/storage"
	"github.com/jterrazz/studio-station/src/fail"
)

func storageFailure(err error) (*fail.Error, bool) {
	var unknown *storage.UnknownToolError
	var place *storage.WrongPlaceError
	var noClean *storage.NoCleanError
	var noReset *storage.NoResetError
	var noName *storage.NoNameError
	switch {
	case errors.As(err, &unknown):
		return fail.Usage(unknown.Error(), "station storage"), true
	case errors.As(err, &place):
		return fail.Usage(place.Error(), "station storage"), true
	case errors.As(err, &noClean):
		return fail.Usage(noClean.Error(), "station storage"), true
	case errors.As(err, &noReset):
		return fail.Usage(noReset.Error(), noReset.Hint), true
	case errors.As(err, &noName):
		return fail.Usage(noName.Error(), "station storage reset <tool...>"), true
	}
	return nil, false
}
