package statuscmd

import (
	"context"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/jterrazz/studio-station/src/cli/kit"
	"github.com/jterrazz/studio-station/src/domain/status"
	"github.com/jterrazz/studio-station/src/page"
	pagehome "github.com/jterrazz/studio-station/src/pages/home"
)

type Deps struct {
	Status *status.Service
	Here   page.Context
}

type Input struct{ All bool }

func Command(d Deps, env kit.Env) *cobra.Command {
	return kit.Command(kit.Leaf[Input]{
		Use:     "status",
		Short:   "Everything about this machine: what needs action first, then each area",
		Example: "  station status\n  station status --all\n  station status --json",
		Group:   "here",
		Declare: func(f *pflag.FlagSet, in *Input) {
			f.BoolVar(&in.All, "all", false, "print every section open, not only what needs action")
		},
		Read:  func(*Input) bool { return true },
		Live:  true,
		Pages: true,
		Run: func(ctx context.Context, in *Input, io kit.IO) error {
			if io.CanLive() {
				var last status.Snapshot
				err := io.LivePage(kit.Pages(d.Status.Watch(ctx), func(s status.Snapshot) page.Page {
					last = s
					return pagehome.Snapshot(s, d.Here, in.All)
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
			return io.Page(pagehome.Snapshot(snapshot, d.Here, in.All), pagehome.SnapshotContract(snapshot))
		},
	}, env)
}
