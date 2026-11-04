package machinescmd

import (
	"context"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/jterrazz/studio-station/src/core/call"
	"github.com/jterrazz/studio-station/src/machines"
	machinescalls "github.com/jterrazz/studio-station/src/machines/calls"
	"github.com/jterrazz/studio-station/src/page"
	pagemachines "github.com/jterrazz/studio-station/src/pages/machines"
	"github.com/jterrazz/studio-station/src/surface/cli/kit"
)

type Deps struct {
	Here page.Context
}

type Input struct{ Here bool }

func Command(d Deps, env kit.Env, children ...*cobra.Command) *cobra.Command {
	cmd := kit.Command(kit.Leaf[Input]{
		Use:     "machines",
		Short:   "Every host and the VMs on each",
		Example: "  station machines\n  station machines --here --json",
		Group:   "fleet",
		Declare: func(f *pflag.FlagSet, in *Input) {
			f.BoolVar(&in.Here, "here", false, "this host only, without asking the others")
		},
		Read:  func(*Input) bool { return true },
		Live:  true,
		Pages: true,
		Run: func(ctx context.Context, in *Input, io kit.IO) error {
			listed := machinescalls.ListIn{Here: in.Here}
			if !io.CanLive() {
				out, err := call.Read(ctx, env.Calls, env.Target(), machinescalls.List, listed, nil)
				if err != nil {
					return err
				}
				return io.Page(listPage(out, d.Here), machines.ToWire(out.Fleet))
			}
			updates, err := call.Follow(ctx, env.Calls, env.Target(), machinescalls.List, listed, nil)
			if err != nil {
				return err
			}
			var failed error
			pages := make(chan page.Page)
			go func() {
				defer close(pages)
				for u := range updates {
					if u.Err != nil {
						failed = u.Err
						continue
					}
					pages <- listPage(u.Value, d.Here)
				}
			}()
			if err := io.LivePage(pages); err != nil {
				return err
			}
			if failed != nil {
				return failed
			}
			return ctx.Err()
		},
	}, env)
	cmd.AddCommand(children...)
	return cmd
}

func listPage(out machinescalls.ListOut, here page.Context) page.Page {
	var warnings []string
	if out.Unrefreshed != nil {
		warnings = append(warnings, "ssh config not refreshed: "+out.Unrefreshed.Error())
	}
	return pagemachines.List(out.Fleet, warnings, here)
}
