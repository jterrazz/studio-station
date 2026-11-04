package stopcmd

import (
	"context"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jterrazz/studio-station/src/core/call"
	"github.com/jterrazz/studio-station/src/core/confirm"
	enginecalls "github.com/jterrazz/studio-station/src/engine/calls"
	"github.com/jterrazz/studio-station/src/page"
	pageoutcome "github.com/jterrazz/studio-station/src/pages/outcome"
	"github.com/jterrazz/studio-station/src/surface/cli/kit"
)

type Deps struct {
	Here page.Context
}

type Input struct {
	// value: what the call said it would end, when it asked.
	asked enginecalls.StopOut
}

func Command(d Deps, env kit.Env) *cobra.Command {
	return kit.Command(kit.Leaf[Input]{
		Use:     "stop",
		Short:   "Stop everything station runs on this host: its VMs (temporary deleted), then station itself",
		Example: "  station stop\n  station stop --yes",
		Group:   kit.ImagesAndEngine,
		Asks:    true,
		Ask: func(in *Input) (bool, error) {
			return env.Prompt.Confirm(pageoutcome.StopConfirm(in.asked.Work, d.Here.Machine))
		},
		Refuse: func(in *Input) error { return kit.Refuse(unconfirmed(in.asked), "station stop --yes") },
		Run: func(ctx context.Context, in *Input, io kit.IO) error {
			var out enginecalls.StopOut
			err := io.Confirmed(func(answer string) (err error) {
				out, err = call.Invoke(ctx, env.Calls, env.Target(), enginecalls.Stop, enginecalls.StopIn{Confirm: answer}, nil)
				if _, asks := confirm.Answer(err); asks {
					in.asked = out
				}
				return err
			})
			if out.Report == nil {
				return err
			}
			if pageErr := io.Page(pageoutcome.Stop(*out.Report, d.Here), nil); pageErr != nil {
				return pageErr
			}
			return err
		},
	}, env)
}

func unconfirmed(out enginecalls.StopOut) string {
	var deletes, stops, parts []string
	for _, v := range out.Work.VMs {
		if v.Result == enginecalls.Deleted {
			deletes = append(deletes, v.Name)
		} else {
			stops = append(stops, v.Name)
		}
	}
	if len(deletes) > 0 {
		parts = append(parts, "deletes "+strings.Join(deletes, ", "))
	}
	if len(stops) > 0 {
		parts = append(parts, "stops "+strings.Join(stops, ", "))
	}
	return "station stop " + strings.Join(parts, " and ")
}
