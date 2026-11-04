package overviewcmd

import (
	"context"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/jterrazz/studio-station/src/core/call"
	"github.com/jterrazz/studio-station/src/machines"
	overviewcalls "github.com/jterrazz/studio-station/src/overview/calls"
	"github.com/jterrazz/studio-station/src/page"
	pagehome "github.com/jterrazz/studio-station/src/pages/home"
	pagemachines "github.com/jterrazz/studio-station/src/pages/machines"

	"github.com/jterrazz/studio-station/src/surface/cli/kit"
)

type Deps struct {
	Here page.Context
	// value: the hidden status alias, whose --json keeps station.status/v1
	// for the scripts that decode it (SPEC §11.4).
	Status bool
	// value: All machines, read once a target names a VM: its Overview is
	// its Mac's view of it; nil reads every target as a machine of its own.
	Fleet func(ctx context.Context) (machines.Fleet, error)
}

type Input struct{ All bool }

func Command(d Deps, env kit.Env) *cobra.Command {
	return kit.Command(kit.Leaf[Input]{
		Use:     "overview",
		Short:   "This machine at a glance: what needs you first, then each section",
		Example: "  station overview\n  station overview --all\n  station overview --json",
		Group:   kit.OnAMachine,
		Declare: func(f *pflag.FlagSet, in *Input) {
			f.BoolVar(&in.All, "all", false, "unfold every section, not only what needs you")
		},
		Read:  func(*Input) bool { return true },
		Live:  true,
		Pages: true,
		Run: func(ctx context.Context, in *Input, io kit.IO) error {
			return Run(ctx, d, env, in.All, io)
		},
	}, env)
}

func Run(ctx context.Context, d Deps, env kit.Env, all bool, io kit.IO) error {
	if vm, ok := d.vm(ctx, env); ok {
		return io.Page(pagemachines.VMPage(vm, d.Here), pagemachines.VMContract(vm, call.Schema(overviewcalls.Read.Name)))
	}
	if d.Status && io.JSON {
		return status(ctx, env, io)
	}
	if io.CanLive() {
		updates, err := call.Follow(ctx, env.Calls, env.Target(), overviewcalls.Read, overviewcalls.ReadIn{}, nil)
		if err != nil {
			return err
		}
		err = io.LivePage(kit.Updates(updates, func(out overviewcalls.ReadOut) page.Page {
			return pagehome.Summary(out.Summary, d.Here, all)
		}))
		if err != nil {
			return err
		}
		return ctx.Err()
	}
	out, err := call.Read(ctx, env.Calls, env.Target(), overviewcalls.Read, overviewcalls.ReadIn{}, nil)
	if err != nil {
		return err
	}
	return io.Page(pagehome.Summary(out.Summary, d.Here, all), pagehome.SummaryContract(out.Summary, call.Schema(overviewcalls.Read.Name)))
}

// value: station status --json, station.status/v1 from the v2 reading,
// for the scripts that decode it (SPEC §11.4).
func status(ctx context.Context, env kit.Env, io kit.IO) error {
	out, err := call.Read(ctx, env.Calls, env.Target(), overviewcalls.Read, overviewcalls.ReadIn{V2: true}, nil)
	if err == nil && out.V2 != nil {
		err = out.V2.Err
	}
	if err != nil {
		return err
	}
	return io.Page(pagehome.Snapshot(*out.V2, page.Context{}, false), pagehome.SnapshotContract(*out.V2))
}

// value: the VM a target names, found in All machines; this machine, or
// another Mac, is none.
func (d Deps) vm(ctx context.Context, env kit.Env) (pagemachines.VMOverview, bool) {
	name := env.Target().Name
	if d.Fleet == nil || name == "" || !env.Far() {
		return pagemachines.VMOverview{}, false
	}
	f, err := d.Fleet(ctx)
	if err != nil {
		return pagemachines.VMOverview{}, false
	}
	h, vm, ok := machines.FindVM(f, name)
	return pagemachines.VMOverview{Fleet: f, Mac: h, VM: vm}, ok
}
