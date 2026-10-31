package app

import (
	"github.com/spf13/cobra"

	"github.com/jterrazz/studio-station/src/cli/kit"
	runcmd "github.com/jterrazz/studio-station/src/cli/run"
	"github.com/jterrazz/studio-station/src/cli/run/docker"
	"github.com/jterrazz/studio-station/src/cli/run/git"
	"github.com/jterrazz/studio-station/src/domain/shortcuts"
)

func runCommand(env kit.Env, m machine) *cobra.Command {
	service := shortcuts.New(m.host)
	return runcmd.Command(env,
		rungit.Command(rungit.Deps{Shortcuts: service}, env),
		rundocker.Command(rundocker.Deps{Shortcuts: service, Context: m.context}, env),
	)
}
