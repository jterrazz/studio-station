package secretsdisallow

import (
	"context"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jterrazz/studio-station/src/accounts"
	accountscalls "github.com/jterrazz/studio-station/src/accounts/calls"
	"github.com/jterrazz/studio-station/src/core/call"
	"github.com/jterrazz/studio-station/src/page"
	pagesecrets "github.com/jterrazz/studio-station/src/pages/secrets"
	"github.com/jterrazz/studio-station/src/surface/cli/kit"
)

type Deps struct {
	// value: the lend list, for completion; the VMs each name is lent to,
	// for the question.
	LendList func() []string
	Held     func(names []string) (map[string][]string, error)
	Here     page.Context
}

func Command(d Deps, env kit.Env) *cobra.Command {
	return kit.Command(kit.Leaf[Input]{
		Use:         "disallow <name...>",
		Short:       "Take names off the lend list and recall their copies from every VM",
		Example:     "  station secrets disallow session:openpanel --yes",
		Args:        cobra.MinimumNArgs(1),
		Bind:        bind,
		Destructive: func(in *Input) bool { return len(in.Held) > 0 },
		ByCall:      true,
		Confirm: func(in *Input) string {
			in.Held, _ = d.Held(in.Names)
			return "Recall " + strings.Join(in.Names, ", ") + " from " + strings.Join(vms(in.Held), ", ") + "?"
		},
		Complete: func([]string, string) []string { return d.LendList() },
		Run: func(ctx context.Context, in *Input, io kit.IO) error {
			in.Held, _ = d.Held(in.Names)
			var report accounts.RecallReport
			err := io.Confirmed(func(answer string) (err error) {
				report, err = call.Invoke(ctx, env.Calls, env.Target, accountscalls.Disallow, accountscalls.DisallowIn{Names: in.Names, Confirm: answer}, nil)
				return err
			})
			if err != nil {
				return err
			}
			if err := io.Page(pagesecrets.Change("disallow", in.Names, report, d.Here), nil); err != nil {
				return err
			}
			if n := len(report.Warnings); n > 0 {
				return kit.PartialFailure{Failed: n, Total: len(in.Held), Noun: "recalls", FirstHint: "re-run it: station secrets recall --from <vm>"}
			}
			return nil
		},
	}, env)
}

func vms(held map[string][]string) []string {
	out := make([]string, 0, len(held))
	for vm := range held {
		out = append(out, vm)
	}
	sort.Strings(out)
	return out
}
