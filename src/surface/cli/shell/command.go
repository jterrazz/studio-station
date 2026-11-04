package shellcmd

import (
	"bytes"
	"context"

	"github.com/spf13/cobra"

	"github.com/jterrazz/studio-station/src/core/call"
	settingscalls "github.com/jterrazz/studio-station/src/settings/calls"
	"github.com/jterrazz/studio-station/src/surface/cli/kit"
)

func Command(env kit.Env) *cobra.Command {
	cmd := kit.Command(kit.Leaf[Input]{
		Use:       "shell <zsh>",
		Short:     "The script the ~/.zshrc block evaluates",
		Args:      cobra.ArbitraryArgs,
		ValidArgs: []string{"zsh"},
		Bind:      bind,
		Run: func(ctx context.Context, _ *Input, io kit.IO) error {
			var completion bytes.Buffer
			if err := io.ZshCompletion(&completion); err != nil {
				return err
			}
			script, err := call.Invoke(ctx, env.Calls, env.Target(), settingscalls.Zsh, settingscalls.ZshIn{Completion: completion.String()}, nil)
			if err != nil {
				return err
			}
			return io.Raw([]byte(script))
		},
	}, env)
	cmd.Hidden = true
	return cmd
}
