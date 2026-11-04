package wire

import (
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/jterrazz/studio-station/src/accounts/adapter/studio"
	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/domain/shell"
	"github.com/jterrazz/studio-station/src/surface/cli/kit"
	shellcmd "github.com/jterrazz/studio-station/src/surface/cli/shell"
)

func shellCommand(env kit.Env, m machine) *cobra.Command {
	service := shell.New(m.host, catalog.DotfilesFS, studio.New(m.host.Run, studio.Options{FileBackend: true, Budget: shell.TokenBudget}), shell.Options{Here: m.kind, Services: filepath.Join(m.paths.Config, "services.env")})
	return shellcmd.Command(shellcmd.Deps{Shell: service}, env)
}
