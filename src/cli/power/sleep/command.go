package powersleep

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/jterrazz/studio-station/src/cli/kit"
	"github.com/jterrazz/studio-station/src/domain/power"
	"github.com/jterrazz/studio-station/src/page"
	pagepower "github.com/jterrazz/studio-station/src/pages/power"
)

type Deps struct {
	Power *power.Service
	Here  page.Context
}

func Command(d Deps, env kit.Env) *cobra.Command {
	return kit.Command(kit.Leaf[struct{}]{
		Use:      "sleep",
		Short:    "End the Power session now: the Mac sleeps as macOS decides",
		Example:  "  station power sleep",
		Validate: func(context.Context, *struct{}) error { return d.Power.Check() },
		Run: func(ctx context.Context, _ *struct{}, io kit.IO) error {
			s, err := d.Power.Sleep(ctx)
			if err != nil {
				return err
			}
			return io.Page(pagepower.Slept(s, d.Here), nil)
		},
	}, env)
}
