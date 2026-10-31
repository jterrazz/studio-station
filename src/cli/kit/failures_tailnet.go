package kit

import (
	"errors"

	"github.com/jterrazz/studio-station/src/domain/tailnet"
	"github.com/jterrazz/studio-station/src/fail"
)

func tailnetFailure(err error) (*fail.Error, bool) {
	var missing *tailnet.NotInstalledError
	var auth *tailnet.AuthError
	var daemon *tailnet.DaemonError
	var noJoin *tailnet.NoVMJoinError
	var needsAuth *tailnet.NeedsAuthError
	var minted *tailnet.MintRefusedError
	var removed *tailnet.RemoveRefusedError
	switch {
	case errors.As(err, &missing):
		return fail.Failed(missing.Error(), "station tools install tailscale", err), true
	case errors.As(err, &auth):
		return fail.Failed(auth.Error(), "station tailnet shows where the node stands", err), true
	case errors.As(err, &daemon):
		return fail.Failed(daemon.Error(), "station tailnet shows where the node stands", err), true
	case errors.As(err, &noJoin):
		return fail.Usage(noJoin.Error(), "station config install vm-join"), true
	case errors.As(err, &needsAuth):
		return fail.Usage(needsAuth.Error(), "run it on a terminal: station tailnet up"), true
	case errors.As(err, &minted):
		return fail.Failed(minted.Error(), "docs/reference/machines.md#vm-join", err), true
	case errors.As(err, &removed):
		return fail.Failed(removed.Error(), "remove it at https://login.tailscale.com/admin/machines", err), true
	}
	return nil, false
}
