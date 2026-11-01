package daemoncmd

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/jterrazz/studio-station/src/cli/kit"
	"github.com/jterrazz/studio-station/src/domain/daemon"
)

type Deps struct{ Daemon *daemon.Daemon }

func Command(d Deps, env kit.Env) *cobra.Command {
	// invariant: plumbing for the host's one launchd agent: it runs until the
	// agent stops it, and answers on its socket, never on stdout.
	return kit.Command(kit.Leaf[struct{}]{
		Use:      "daemon",
		Short:    "Run this host's daemon: it supervises the VMs and publishes their services",
		Plumbing: true,
		Validate: func(context.Context, *struct{}) error { return d.Daemon.Check() },
		Run:      func(ctx context.Context, _ *struct{}, io kit.IO) error { return d.Daemon.Run(ctx, io.Note) },
	}, env)
}
