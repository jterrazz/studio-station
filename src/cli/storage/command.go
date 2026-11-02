package storagecmd

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/jterrazz/studio-station/src/cli/kit"
	"github.com/jterrazz/studio-station/src/domain/storage"
	"github.com/jterrazz/studio-station/src/present"
	presentstorage "github.com/jterrazz/studio-station/src/present/storage"
)

type Deps struct {
	Storage *storage.Service
	Context string
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
		Entry:   kit.Entry("station storage clean", d.Storage.Names),
		Run: func(ctx context.Context, _ *Input, io kit.IO) error {
			if io.CanLive() {
				if err := io.Live(kit.Answers(d.Storage.Watch(ctx), func(r storage.Report) present.Answer {
					return presentstorage.List(r, d.Context)
				})); err != nil {
					return err
				}
				return ctx.Err()
			}
			report, err := d.Storage.List(ctx)
			if err != nil {
				return err
			}
			return io.Answer(presentstorage.List(report, d.Context))
		},
	}, env)
	cmd.AddCommand(children...)
	return cmd
}
