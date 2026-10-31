package setupcmd

import (
	"context"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/jterrazz/studio-station/src/cli/kit"
	"github.com/jterrazz/studio-station/src/domain/setup"
	presentsetup "github.com/jterrazz/studio-station/src/present/setup"
)

type Deps struct {
	Setup   *setup.Service
	Context string
}

func Command(d Deps, env kit.Env) *cobra.Command {
	return kit.Command(kit.Leaf[Input]{
		Use:     "setup",
		Short:   "Set this machine up: its name, the ~/.zshrc block, the XDG folders",
		Example: "  station setup\n  station setup --name macbook",
		Group:   "binary",
		Declare: func(f *pflag.FlagSet, in *Input) {
			f.StringVar(&in.Name, "name", "", "name this host in the fleet (asked on a terminal when omitted)")
		},
		Validate: func(_ context.Context, in *Input) error { return d.Setup.CheckName(in.Name) },
		Run: func(ctx context.Context, in *Input, io kit.IO) error {
			report, err := d.Setup.Run(ctx, setup.Request{
				Name:        in.Name,
				Interactive: io.Interactive,
				AskName:     func() (string, error) { return io.Prompt.AskName(d.Setup.DefaultName(ctx)) },
			})
			if len(report.Steps) > 0 {
				if answerErr := io.Answer(presentsetup.Outcome(report, d.Context)); answerErr != nil {
					return answerErr
				}
			}
			return err
		},
	}, env)
}
