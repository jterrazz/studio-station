package app

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/jterrazz/studio-station/src/cli/kit"
)

func tuiOpener(env kit.Env, root func() *cobra.Command) func(context.Context) error {
	return func(context.Context) error { return kit.Menu(env, root()) }
}
