package settingscmd

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/core/call"
	settingscalls "github.com/jterrazz/studio-station/src/settings/calls"
	"github.com/jterrazz/studio-station/src/surface/cli/kit"
)

// value: v2's run git shortcut word of an alias of the Git shortcuts; the
// others kept their word.
func oldGitWord(alias string) string {
	switch alias {
	case "lg":
		return "log"
	case "pushup":
		return "push"
	case "st":
		return "status"
	}
	return alias
}

type runGitIn struct{ Words []string }

func RunGit(env kit.Env) *cobra.Command {
	// value: v2's run git <shortcut>, now git's own alias (SPEC §4.3): it
	// runs that alias through the Git shortcuts setting.
	leaves := make([]*cobra.Command, 0, len(catalog.GitShortcuts.Aliases))
	for _, a := range catalog.GitShortcuts.Aliases {
		leaves = append(leaves, gitLeaf(env, oldGitWord(a.Name), a.Name))
	}
	return kit.Group("git", "Git shortcuts, now git's own aliases", "", env, leaves...)
}

func gitLeaf(env kit.Env, old, alias string) *cobra.Command {
	return kit.AliasShell(kit.Command(kit.Leaf[runGitIn]{
		Use:     old,
		Short:   "git " + alias,
		Args:    cobra.ArbitraryArgs,
		RawArgs: true,
		Bind: func(in *runGitIn, args []string) error {
			in.Words = args
			return nil
		},
		Run: func(ctx context.Context, in *runGitIn, _ kit.IO) error {
			_, err := call.Invoke(ctx, env.Calls, env.Target(), settingscalls.Git, settingscalls.GitIn{Alias: alias, Words: in.Words}, nil)
			return err
		},
	}, env), "git "+alias)
}
