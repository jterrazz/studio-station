package statuscmd

import (
	"context"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/jterrazz/studio-station/src/core/call"
	overviewcalls "github.com/jterrazz/studio-station/src/overview/calls"
	"github.com/jterrazz/studio-station/src/page"
	pagehome "github.com/jterrazz/studio-station/src/pages/home"
	"github.com/jterrazz/studio-station/src/surface/cli/kit"
)

type Deps struct {
	Here page.Context
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
				updates, err := call.Follow(ctx, env.Calls, env.Target, overviewcalls.Read, overviewcalls.ReadIn{}, nil)
				if err != nil {
					return err
				}
				var last pagehome.Overview
				err = io.LivePage(kit.Updates(updates, func(s pagehome.Overview) page.Page {
					last = s
					return pagehome.Snapshot(s, d.Here, in.All)
				}))
				if err != nil {
					return err
				}
				if err := ctx.Err(); err != nil {
					return err
				}
				return last.Err
			}
			snapshot, err := call.Read(ctx, env.Calls, env.Target, overviewcalls.Read, overviewcalls.ReadIn{}, nil)
			if err == nil {
				err = snapshot.Err
			}
			if err != nil {
				return err
			}
			return io.Page(pagehome.Snapshot(snapshot, d.Here, in.All), pagehome.SnapshotContract(snapshot))
		},
	}, env)
}
