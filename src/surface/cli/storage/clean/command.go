package storageclean

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
	Names func() []string
	Here  page.Context
}

type Input struct{ Names []string }

func Command(d Deps, env kit.Env) *cobra.Command {
	return kit.Command(kit.Leaf[Input]{
		Use:      "clean [<tool...>]",
		Short:    "Empty the caches of the named tools, or of every tool here, with their own commands",
		Example:  "  station storage clean\n  station storage clean go node",
		Args:     cobra.ArbitraryArgs,
		Bind:     func(in *Input, args []string) error { in.Names = args; return nil },
		Complete: func([]string, string) []string { return d.Names() },
		Run: func(ctx context.Context, in *Input, io kit.IO) error {
			outcomes, err := call.Invoke(ctx, env.Calls, env.Target, diskcalls.Clean, diskcalls.CleanIn{Names: in.Names}, nil)
			if err != nil {
				return err
			}
			if err := io.Page(pagestorage.Outcomes("clean", outcomes, d.Here), nil); err != nil {
				return err
			}
			return partial(outcomes)
		},
	}, env)
}

func partial(outcomes []disk.Outcome) error {
	failed, first := disk.Failures(outcomes)
	if failed == 0 {
		return nil
	}
	hint := first.Hint
	if hint == "" {
		hint = "re-run it: station storage clean " + first.Name
	}
	return kit.PartialFailure{Failed: failed, Total: len(outcomes), Noun: "tools", FirstHint: hint}
}
