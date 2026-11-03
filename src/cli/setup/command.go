package setupcmd

import (
	"context"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/jterrazz/studio-station/src/cli/kit"
	"github.com/jterrazz/studio-station/src/domain/setup"
	"github.com/jterrazz/studio-station/src/page"
	pagesetup "github.com/jterrazz/studio-station/src/pages/setup"
)

type Deps struct {
	Setup *setup.Service
	Here  page.Context
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
				AskName: func() (string, error) {
					values, err := io.Prompt.Form(pagesetup.NameForm(d.Setup.DefaultName(ctx)))
					return strings.TrimSpace(values["name"]), err
				},
			})
			if len(report.Steps) > 0 {
				if answerErr := io.Page(pagesetup.Outcome(report, d.Here), nil); answerErr != nil {
					return answerErr
				}
			}
			return err
		},
	}, env)
}
