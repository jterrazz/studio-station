package machinesadd

import (
	"context"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/jterrazz/studio-station/src/domain/machines"
	"github.com/jterrazz/studio-station/src/domain/settings"
	"github.com/jterrazz/studio-station/src/page"
	pagemachines "github.com/jterrazz/studio-station/src/pages/machines"
	"github.com/jterrazz/studio-station/src/surface/cli/kit"
)

type Deps struct {
	Machines *machines.Service
	Here     page.Context
}

type Input struct{ Name, User, Key string }

func Command(d Deps, env kit.Env) *cobra.Command {
	return kit.Command(kit.Leaf[Input]{
		Use:     "add",
		Short:   "Register another host: its name, and the login and key ssh uses",
		Example: "  station machines add --name mac-mini --user jterrazz.agent --key ~/.ssh/id_ed25519",
		Declare: func(f *pflag.FlagSet, in *Input) {
			f.StringVar(&in.Name, "name", "", "the host's name, its ssh Host and its tailnet name (required)")
			f.StringVar(&in.User, "user", "", "the login on that host, when it differs from yours")
			f.StringVar(&in.Key, "key", "", "the ssh key file (default ~/.ssh/id_ed25519)")
		},
		Bind: func(in *Input, _ []string) error {
			if in.Name == "" {
				return kit.Refuse("--name is required", "station machines add --name <name>")
			}
			return nil
		},
		Validate: func(context.Context, *Input) error { return d.Machines.CheckHost() },
		Run: func(_ context.Context, in *Input, io kit.IO) error {
			if err := d.Machines.AddHost(in.Name, settings.Host{User: in.User, Key: in.Key}); err != nil {
				return err
			}
			return io.Page(pagemachines.Change("add", in.Name, "added, ssh "+in.Name, d.Here), nil)
		},
	}, env)
}
