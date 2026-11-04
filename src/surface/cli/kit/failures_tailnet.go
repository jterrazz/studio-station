package kit

import (
	"errors"

	"github.com/jterrazz/studio-station/src/core/fail"
	"github.com/jterrazz/studio-station/src/ports"
)

func tailnetFailure(err error) (*fail.Error, bool) {
	var absent *ports.Absent
	var refused *ports.Refused
	switch {
	case errors.As(err, &absent):
		return failed(err.Error(), "station apps install "+absent.App, err), true
	case errors.As(err, &refused):
		return failed(refused.Error(), "docs/reference/services.md#https", err), true
	}
	return nil, false
}
