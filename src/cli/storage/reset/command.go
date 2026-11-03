package storagereset

import (
	"context"
	"strings"

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

type Input struct{ Names []string }

func Command(d Deps, env kit.Env) *cobra.Command {
	return kit.Command(kit.Leaf[Input]{
		Use:      "reset <tool...>",
		Short:    "Delete the named tools' data and caches, back to factory; asks first",
		Example:  "  station storage reset zed\n  station storage reset xcode --yes",
		Args:     cobra.ArbitraryArgs,
		Bind:     func(in *Input, args []string) error { in.Names = args; return nil },
		Validate: func(_ context.Context, in *Input) error { _, err := d.Storage.CheckReset(in.Names); return err },
		Complete: func([]string, string) []string {
			return d.Storage.Names()
		},
		Destructive: func(*Input) bool { return true },
		Confirm:     func(in *Input) string { return "Reset " + strings.Join(in.Names, ", ") + "?" },
		Ask: func(in *Input) (bool, error) {
			return env.Prompt.Confirm(pagestorage.ResetConfirm(in.Names, d.Storage.Plan(in.Names)))
		},
		Run: func(ctx context.Context, in *Input, io kit.IO) error {
			outcomes, err := d.Storage.Reset(ctx, in.Names)
			if err != nil {
				return err
			}
			if err := io.Page(pagestorage.Outcomes("reset", outcomes, d.Here), nil); err != nil {
				return err
			}
			failed, first := storage.Failures(outcomes)
			if failed == 0 {
				return nil
			}
			hint := first.Hint
			if hint == "" {
				hint = "re-run it: station storage reset " + first.Name + " --yes"
			}
			return kit.PartialFailure{Failed: failed, Total: len(outcomes), Noun: "tools", FirstHint: hint}
		},
	}, env)
}
