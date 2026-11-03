package stopcmd

import (
	"context"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/jterrazz/studio-station/src/cli/kit"
	"github.com/jterrazz/studio-station/src/domain/daemon"
	"github.com/jterrazz/studio-station/src/page"
	pageoutcome "github.com/jterrazz/studio-station/src/pages/outcome"
)

type Deps struct {
	Control *daemon.Control
	Here    page.Context
}

type Input struct{ Yes bool }

func Command(d Deps, env kit.Env) *cobra.Command {
	return kit.Command(kit.Leaf[Input]{
		Use:     "stop",
		Short:   "Stop everything station runs on this host: its VMs (temporary deleted), then station itself",
		Example: "  station stop\n  station stop --yes",
		Group:   "fleet",
		Declare: func(f *pflag.FlagSet, in *Input) {
			f.BoolVarP(&in.Yes, "yes", "y", false, "do it without asking")
		},
		Validate: func(context.Context, *Input) error { return d.Control.Check("stop") },
		Run: func(ctx context.Context, in *Input, io kit.IO) error {
			work, err := d.Control.Plan(ctx)
			if err != nil {
				return err
			}
			if !work.Empty() && !in.Yes {
				if !io.Interactive {
					return unconfirmed(work)
				}
				ask := func() (bool, error) { return io.Prompt.Confirm(pageoutcome.StopConfirm(work, d.Here.Machine)) }
				if askErr := kit.Ask("stop", ask, io); askErr != nil {
					return askErr
				}
			}
			report, err := d.Control.Stop(ctx, work)
			if pageErr := io.Page(pageoutcome.Stop(report, d.Here), nil); pageErr != nil {
				return pageErr
			}
			return err
		},
	}, env)
}

func unconfirmed(w daemon.Work) error {
	e := &daemon.UnconfirmedError{}
	for _, v := range w.VMs {
		if v.Result == daemon.ResultDeleted {
			e.Deletes = append(e.Deletes, v.Name)
		} else {
			e.Stops = append(e.Stops, v.Name)
		}
	}
	return e
}
