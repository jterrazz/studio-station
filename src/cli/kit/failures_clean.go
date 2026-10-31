package kit

import (
	"errors"

	"github.com/jterrazz/studio-station/src/domain/clean"
	"github.com/jterrazz/studio-station/src/fail"
)

func cleanFailure(err error) (*fail.Error, bool) {
	var unknown *clean.UnknownCleanableError
	var place *clean.WrongPlaceError
	switch {
	case errors.As(err, &unknown):
		return fail.Usage(unknown.Error(), "run station clean for what can be reclaimed here"), true
	case errors.As(err, &place):
		return fail.Usage(place.Error(), "run station clean for what can be reclaimed here"), true
	}
	return nil, false
}
