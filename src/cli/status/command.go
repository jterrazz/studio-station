package statuscmd

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/jterrazz/studio-station/src/cli/kit"
	"github.com/jterrazz/studio-station/src/domain/status"
	"github.com/jterrazz/studio-station/src/present"
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
		Live:    true,
		Run: func(ctx context.Context, _ *struct{}, io kit.IO) error {
			if io.CanLive() {
				var last status.Snapshot
				err := io.Live(kit.Answers(d.Status.Watch(ctx), func(s status.Snapshot) present.Answer {
					last = s
					return presentstatus.Snapshot(s, d.Context)
				}))
				if err != nil {
					return err
				}
				if err := ctx.Err(); err != nil {
					return err
				}
				return last.Err()
			}
			snapshot, err := d.Status.Snapshot(ctx)
			if err != nil {
				return err
			}
			return io.Answer(presentstatus.Snapshot(snapshot, d.Context))
		},
	}, env)
}
