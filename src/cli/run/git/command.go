package rungit

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/cli/kit"
	"github.com/jterrazz/studio-station/src/domain/shortcuts"
)

type Deps struct{ Shortcuts *shortcuts.Service }

func Command(d Deps, env kit.Env) *cobra.Command {
	leaves := make([]*cobra.Command, 0, len(catalog.GitShortcuts))
	for _, sc := range catalog.GitShortcuts {
		leaves = append(leaves, leaf(d, env, sc))
	}
	return kit.Group("git", "Git workflow shortcuts", "", env, leaves...)
}

func leaf(d Deps, env kit.Env, sc catalog.GitShortcut) *cobra.Command {
	use, args, example := sc.Name, cobra.PositionalArgs(cobra.NoArgs), "  station run git "+sc.Name
	if sc.MinArgs > 0 {
		use, args, example = sc.Name+" <message...>", cobra.MinimumNArgs(sc.MinArgs), "  station run git "+sc.Name+" add the login page"
	}
	return kit.Command(kit.Leaf[Input]{
		Use:     use,
		Short:   sc.Description,
		Example: example,
		Args:    args,
		RawArgs: true,
		Bind:    bind,
		Run: func(ctx context.Context, in *Input, _ kit.IO) error {
			return d.Shortcuts.Git(ctx, sc.Name, in.Words)
		},
	}, env)
}
