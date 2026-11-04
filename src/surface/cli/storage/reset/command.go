package storagereset

import (
	"context"
	"strings"

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
	// value: what a reset would delete, for its question.
	Plan func(names []string) []disk.PathReport
	Here page.Context
}

type Input struct{ Names []string }

func Command(d Deps, env kit.Env) *cobra.Command {
	return kit.Command(kit.Leaf[Input]{
		Use:      "reset <tool...>",
		Short:    "Delete the named tools' data and caches, back to factory; asks first",
		Example:  "  station storage reset zed\n  station storage reset xcode --yes",
		Args:     cobra.ArbitraryArgs,
		Bind:     func(in *Input, args []string) error { in.Names = args; return nil },
		Complete: func([]string, string) []string { return d.Names() },
		Asks:     true,
		Confirm:  func(in *Input) string { return "Reset " + strings.Join(in.Names, ", ") + "?" },
		Ask: func(in *Input) (bool, error) {
			return env.Prompt.Confirm(pagestorage.ResetConfirm(in.Names, d.Plan(in.Names)))
		},
		Run: func(ctx context.Context, in *Input, io kit.IO) error {
			var outcomes []disk.Outcome
			err := io.Confirmed(func(answer string) (err error) {
				outcomes, err = call.Invoke(ctx, env.Calls, env.Target(), diskcalls.Reset, diskcalls.ResetIn{Names: in.Names, Confirm: answer}, nil)
				return err
			})
			if err != nil {
				return err
			}
			if err := io.Page(pagestorage.Outcomes("reset", outcomes, d.Here), nil); err != nil {
				return err
			}
			failed, first := disk.Failures(outcomes)
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
