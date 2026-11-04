package overviewcmd

import (
	"context"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/jterrazz/studio-station/src/core/call"
	overviewcalls "github.com/jterrazz/studio-station/src/overview/calls"
	"github.com/jterrazz/studio-station/src/page"
	pagehome "github.com/jterrazz/studio-station/src/pages/home"
	"github.com/jterrazz/studio-station/src/surface/cli/kit"
)

type Deps struct {
	Here page.Context
	// value: the hidden status alias, whose --json keeps station.status/v1
	// for the scripts that decode it (SPEC §11.4).
	Status bool
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
	if io.CanLive() {
		updates, err := call.Follow(ctx, env.Calls, env.Target(), overviewcalls.Read, overviewcalls.ReadIn{}, nil)
		if err != nil {
			return err
		}
		var last pagehome.Overview
		err = io.LivePage(kit.Updates(updates, func(s pagehome.Overview) page.Page {
			last = s
			return pagehome.Snapshot(s, d.Here, all)
		}))
		if err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		return last.Err
	}
	snapshot, err := call.Read(ctx, env.Calls, env.Target(), overviewcalls.Read, overviewcalls.ReadIn{}, nil)
	if err == nil {
		err = snapshot.Err
	}
	if err != nil {
		return err
	}
	contract := pagehome.SnapshotContract(snapshot)
	if !d.Status {
		contract.Schema = call.Schema(overviewcalls.Read.Name)
	}
	return io.Page(pagehome.Snapshot(snapshot, d.Here, all), contract)
}
