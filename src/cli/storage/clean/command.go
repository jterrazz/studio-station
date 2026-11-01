package storageclean

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/jterrazz/studio-station/src/cli/kit"
	"github.com/jterrazz/studio-station/src/domain/storage"
	presentstorage "github.com/jterrazz/studio-station/src/present/storage"
)

type Deps struct {
	Storage *storage.Service
	Context string
}

type Input struct{ Names []string }

func Command(d Deps, env kit.Env) *cobra.Command {
	return kit.Command(kit.Leaf[Input]{
		Use:     "clean [<tool...>]",
		Short:   "Empty the caches of the named tools, or of every tool here, with their own commands",
		Example: "  station storage clean\n  station storage clean go node",
		Args:    cobra.ArbitraryArgs,
		Bind:    func(in *Input, args []string) error { in.Names = args; return nil },
		Complete: func([]string, string) []string {
			return d.Storage.Names()
		},
		Run: func(ctx context.Context, in *Input, io kit.IO) error {
			outcomes, err := d.Storage.Clean(ctx, in.Names)
			if err != nil {
				return err
			}
			if err := io.Answer(presentstorage.Outcomes("clean", outcomes, d.Context)); err != nil {
				return err
			}
			return partial(outcomes)
		},
	}, env)
}

func partial(outcomes []storage.Outcome) error {
	failed, first := storage.Failures(outcomes)
	if failed == 0 {
		return nil
	}
	hint := first.Hint
	if hint == "" {
		hint = "re-run it: station storage clean " + first.Name
	}
	return kit.PartialFailure{Failed: failed, Total: len(outcomes), Noun: "tools", FirstHint: hint}
}
