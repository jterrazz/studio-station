package storagecmd

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/jterrazz/studio-station/src/cli/kit"
	"github.com/jterrazz/studio-station/src/domain/storage"
	"github.com/jterrazz/studio-station/src/page"
	pagestorage "github.com/jterrazz/studio-station/src/pages/storage"
)

type Deps struct {
	Storage *storage.Service
	Here    page.Context
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
		Entry:   kit.Entry("station storage clean", d.Storage.Names),
		Run: func(ctx context.Context, _ *Input, io kit.IO) error {
			if io.CanLive() {
				if err := io.LivePage(kit.Pages(d.Storage.Watch(ctx), func(r storage.Report) page.Page {
					return pagestorage.List(r, d.Here)
				})); err != nil {
					return err
				}
				return ctx.Err()
			}
			report, err := d.Storage.List(ctx)
			if err != nil {
				return err
			}
			return io.Page(pagestorage.List(report, d.Here), pagestorage.ListContract(report))
		},
	}, env)
	cmd.AddCommand(children...)
	return cmd
}
