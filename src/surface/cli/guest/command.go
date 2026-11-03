package guestcmd

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/jterrazz/studio-station/src/domain/guest"
	"github.com/jterrazz/studio-station/src/surface/cli/kit"
)

type Deps struct{ Guest *guest.Service }

func Command(d Deps, env kit.Env) *cobra.Command {
	// invariant: plumbing run by the host's daemon inside each VM: its stdin and
	// stdout are the host's channel.
	return kit.Command(kit.Leaf[struct{}]{
		Use:      "guest",
		Short:    "Report this VM's listening ports to its host's daemon",
		Plumbing: true,
		Validate: func(context.Context, *struct{}) error { return d.Guest.Check() },
		Run: func(ctx context.Context, _ *struct{}, io kit.IO) error {
			in, out := io.Streams()
			return d.Guest.Run(ctx, in, out)
		},
	}, env)
}
