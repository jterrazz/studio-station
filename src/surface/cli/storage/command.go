package storagecmd

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/jterrazz/studio-station/src/core/call"
	"github.com/jterrazz/studio-station/src/disk"
	diskcalls "github.com/jterrazz/studio-station/src/disk/calls"
	"github.com/jterrazz/studio-station/src/page"
	pagestorage "github.com/jterrazz/studio-station/src/pages/storage"
	"github.com/jterrazz/studio-station/src/surface/cli/kit"
)

type Deps struct {
	// value: the tools whose storage is placed here, for completion.
	Names func() []string
	Here  page.Context
}

type Input struct{}

func Command(d Deps, env kit.Env, children ...*cobra.Command) *cobra.Command {
	cmd := kit.Command(kit.Leaf[Input]{
		Use:     "storage",
		Short:   "What each tool keeps on this machine, by level: caches, data, kept, installed",
		Example: "  station storage\n  station storage --json",
		Group:   "here",
		Read:    func(*Input) bool { return true },
		Live:    true,
		Pages:   true,
		Entry:   kit.Entry("station storage clean", d.Names),
		Run: func(ctx context.Context, _ *Input, io kit.IO) error {
			if io.CanLive() {
				updates, err := call.Follow(ctx, env.Calls, env.Target(), diskcalls.List, diskcalls.ListIn{}, nil)
				if err != nil {
					return err
				}
				if err := io.LivePage(kit.Updates(updates, func(r disk.Report) page.Page {
					return pagestorage.List(r, d.Here)
				})); err != nil {
					return err
				}
				return ctx.Err()
			}
			report, err := call.Read(ctx, env.Calls, env.Target(), diskcalls.List, diskcalls.ListIn{}, nil)
			if err != nil {
				return err
			}
			return io.Page(pagestorage.List(report, d.Here), pagestorage.ListContract(report))
		},
	}, env)
	cmd.AddCommand(children...)
	return cmd
}
