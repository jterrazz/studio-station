package statuscmd

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/jterrazz/studio-station/src/cli/kit"
	"github.com/jterrazz/studio-station/src/domain/status"
	presentstatus "github.com/jterrazz/studio-station/src/present/status"
)

type Deps struct {
	Status  *status.Service
	Context string
}

func Command(d Deps, env kit.Env) *cobra.Command {
	return kit.Command(kit.Leaf[struct{}]{
		Use:     "status",
		Short:   "Everything about this machine: system, environment, tools, config, setup, gateways",
		Example: "  station status\n  station status --json",
		Group:   "here",
		Read:    func(*struct{}) bool { return true },
		Run: func(ctx context.Context, _ *struct{}, io kit.IO) error {
			snapshot, err := d.Status.Snapshot(ctx)
			if err != nil {
				return err
			}
			return io.Answer(presentstatus.Snapshot(snapshot, d.Context))
		},
	}, env)
}
