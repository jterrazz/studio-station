package app

import (
	"github.com/spf13/cobra"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/cli/kit"
	shellcmd "github.com/jterrazz/studio-station/src/cli/shell"
	"github.com/jterrazz/studio-station/src/domain/shell"
)

func shellCommand(env kit.Env, m machine) *cobra.Command {
	service := shell.New(m.host, catalog.DotfilesFS, shell.Options{Here: m.kind})
	return shellcmd.Command(shellcmd.Deps{Shell: service}, env)
}
