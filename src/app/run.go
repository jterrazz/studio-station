package app

import (
	"github.com/spf13/cobra"

	"github.com/jterrazz/studio-station/src/cli/kit"
)

func runCommand(env kit.Env) *cobra.Command {
	return kit.Group("run", "Canned git and docker command sequences", "shortcuts", env)
}
