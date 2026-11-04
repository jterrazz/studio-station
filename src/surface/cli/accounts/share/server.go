package accountsshare

import (
	"context"
	"errors"

	accountscalls "github.com/jterrazz/studio-station/src/accounts/calls"
	"github.com/jterrazz/studio-station/src/core/call"
	"github.com/jterrazz/studio-station/src/core/fail"
	"github.com/jterrazz/studio-station/src/core/machine"
	"github.com/jterrazz/studio-station/src/surface/cli/kit"
)

// invariant: a server keeps its own secrets: a running VM whose own machine
// says its Accounts section is not for it is a server.
func isServer(ctx context.Context, env kit.Env, h kit.Held) bool {
	if !h.Running {
		return false
	}
	_, err := call.Read(ctx, env.Calls, machine.Ref{Name: h.VM, On: h.Mac}, accountscalls.List, accountscalls.ListIn{}, nil)
	var refused *fail.Error
	return errors.As(err, &refused) && refused.Type == fail.NotForThisMachine
}
