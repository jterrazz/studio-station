package enginecmd

import (
	"github.com/spf13/cobra"

	"github.com/jterrazz/studio-station/src/surface/cli/kit"
)

func Command(env kit.Env, children ...*cobra.Command) *cobra.Command {
	group := kit.Group("engine", "This Mac's engine: the API another Mac reaches", "", env, children...)
	group.Hidden = true
	return group
}
