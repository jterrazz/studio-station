package runcmd

import (
	"github.com/spf13/cobra"

	"github.com/jterrazz/studio-station/src/surface/cli/kit"
)

func Command(env kit.Env, children ...*cobra.Command) *cobra.Command {
	return kit.Group("run", "Canned git and docker command sequences", kit.OnAMachine, env, children...)
}
