package startcmd

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/jterrazz/studio-station/src/cli/kit"
	"github.com/jterrazz/studio-station/src/engine"
	"github.com/jterrazz/studio-station/src/page"
	pageoutcome "github.com/jterrazz/studio-station/src/pages/outcome"
)

type Deps struct {
	Control *engine.Control
	Here    page.Context
}

type Input struct{}

func Command(d Deps, env kit.Env) *cobra.Command {
	return kit.Command(kit.Leaf[Input]{
		Use:      "start",
		Short:    "Start station on this host again, and the lasting VMs whose wish is running",
		Example:  "  station start",
		Group:    "fleet",
		Validate: func(context.Context, *Input) error { return d.Control.Check("start") },
		Run: func(ctx context.Context, _ *Input, io kit.IO) error {
			report, err := d.Control.Start(ctx)
			if err != nil {
				return err
			}
			return io.Page(pageoutcome.Start(report, d.Here), nil)
		},
	}, env)
}
