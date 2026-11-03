package wire

import (
	"github.com/spf13/cobra"

	"github.com/jterrazz/studio-station/src/domain/shortcuts"
	"github.com/jterrazz/studio-station/src/surface/cli/kit"
	runcmd "github.com/jterrazz/studio-station/src/surface/cli/run"
	"github.com/jterrazz/studio-station/src/surface/cli/run/docker"
	"github.com/jterrazz/studio-station/src/surface/cli/run/git"
)

func runCommand(env kit.Env, m machine) *cobra.Command {
	service := shortcuts.New(m.host)
	return runcmd.Command(env,
		rungit.Command(rungit.Deps{Shortcuts: service}, env),
		rundocker.Command(rundocker.Deps{Shortcuts: service, Here: m.where}, env),
	)
}
