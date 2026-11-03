package kit

import (
	"strconv"

	"github.com/jterrazz/studio-station/src/core/fail"
)

type PartialFailure struct {
	Failed, Total int
	Noun          string
	FirstHint     string
}

func (p PartialFailure) Error() string {
	return strconv.Itoa(p.Failed) + " of " + strconv.Itoa(p.Total) + " " + p.Noun + " failed"
}

func (p PartialFailure) As(target any) bool {
	t, ok := target.(**fail.Error)
	if ok {
		*t = fail.Failed(p.Error(), p.FirstHint, nil)
	}
	return ok
}
