package secretsdisallow

import (
	"context"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jterrazz/studio-station/src/cli/kit"
	"github.com/jterrazz/studio-station/src/domain/secrets"
	presentsecrets "github.com/jterrazz/studio-station/src/present/secrets"
)

type Deps struct {
	Secrets *secrets.Service
	Context string
}

func Command(d Deps, env kit.Env) *cobra.Command {
	return kit.Command(kit.Leaf[Input]{
		Use:     "disallow <name...>",
		Short:   "Take names off the lend list and recall their copies from every VM",
		Example: "  station secrets disallow session:openpanel --yes",
		Args:    cobra.MinimumNArgs(1),
		Bind:    bind,
		Validate: func(_ context.Context, in *Input) error {
			if err := d.Secrets.CheckDisallow(in.Names); err != nil {
				return err
			}
			held, err := d.Secrets.Held(in.Names)
			in.Held = held
			return err
		},
		Destructive: func(in *Input) bool { return len(in.Held) > 0 },
		Confirm: func(in *Input) string {
			return "Recall " + strings.Join(in.Names, ", ") + " from " + strings.Join(vms(in.Held), ", ") + "?"
		},
		Complete: func([]string, string) []string { return d.Secrets.LendList() },
		Run: func(ctx context.Context, in *Input, io kit.IO) error {
			report, err := d.Secrets.Disallow(ctx, in.Names)
			if err != nil {
				return err
			}
			if err := io.Answer(presentsecrets.Change("disallow", in.Names, report, d.Context)); err != nil {
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
