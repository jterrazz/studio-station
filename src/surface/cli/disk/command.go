package diskcmd

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/jterrazz/studio-station/src/core/call"
	"github.com/jterrazz/studio-station/src/disk"
	diskcalls "github.com/jterrazz/studio-station/src/disk/calls"
	"github.com/jterrazz/studio-station/src/page"
	pagedisk "github.com/jterrazz/studio-station/src/pages/disk"
	"github.com/jterrazz/studio-station/src/surface/cli/kit"
)

type Deps struct {
	// value: the apps with a Disk row here, for completion.
	Names func() []string
	Here  page.Context
}

type Input struct{}

func Command(d Deps, env kit.Env, children ...*cobra.Command) *cobra.Command {
	cmd := kit.Command(kit.Leaf[Input]{
		Use:     "disk",
		Short:   "What each app keeps on this machine: misplaced, can rebuild, your data, never touched",
		Example: "  station disk\n  station disk --json",
		Group:   kit.OnAMachine,
		Read:    func(*Input) bool { return true },
		Live:    true,
		Pages:   true,
		Entry:   kit.Entry("station disk free", "disk row", d.Names),
		Run: func(ctx context.Context, _ *Input, io kit.IO) error {
			here := d.Here
			if env.Far() {
				here = page.Context{Machine: env.Target().Name}
			}
			if io.CanLive() {
				updates, err := call.Follow(ctx, env.Calls, env.Target(), diskcalls.Read, diskcalls.ReadIn{}, nil)
				if err != nil {
					return err
				}
				if err := io.LivePage(kit.Updates(updates, func(r disk.Reading) page.Page {
					return pagedisk.Read(r, here)
				})); err != nil {
					return err
				}
				return ctx.Err()
			}
			reading, err := call.Read(ctx, env.Calls, env.Target(), diskcalls.Read, diskcalls.ReadIn{}, nil)
			if err != nil {
				return err
			}
			return io.Page(pagedisk.Read(reading, here), pagedisk.ReadContract(reading))
		},
	}, env)
	cmd.AddCommand(children...)
	return cmd
}
