package kit

import (
	"errors"

	"github.com/jterrazz/studio-station/src/domain/tailnet"
	"github.com/jterrazz/studio-station/src/fail"
)

func tailnetFailure(err error) (*fail.Error, bool) {
	var serve *tailnet.ServeError
	switch {
	case errors.Is(err, tailnet.ErrAbsent):
		return fail.Failed(err.Error(), "station tools install tailscale", err), true
	case errors.As(err, &serve):
		return fail.Failed(serve.Error(), "docs/reference/services.md#https", err), true
	}
	return nil, false
}
