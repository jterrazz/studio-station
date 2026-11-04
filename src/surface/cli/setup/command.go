package setupcmd

import (
	"context"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/jterrazz/studio-station/src/core/call"
	"github.com/jterrazz/studio-station/src/page"
	pagesetup "github.com/jterrazz/studio-station/src/pages/setup"
	setupcalls "github.com/jterrazz/studio-station/src/setup/calls"
	"github.com/jterrazz/studio-station/src/surface/cli/kit"
)

type Deps struct {
	Here page.Context
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
		Run: func(ctx context.Context, in *Input, io kit.IO) error {
			report, err := call.Invoke(ctx, env.Calls, env.Target, setupcalls.Run, setupcalls.RunIn{Name: in.Name, Interactive: io.Interactive}, nil)
			if len(report.Steps) > 0 {
				if answerErr := io.Page(pagesetup.Outcome(report, d.Here), nil); answerErr != nil {
					return answerErr
				}
			}
			return err
		},
	}, env)
}
