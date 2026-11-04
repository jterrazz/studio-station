package calls

import (
	"context"

	"github.com/jterrazz/studio-station/src/core/call"
	"github.com/jterrazz/studio-station/src/core/machine"
	"github.com/jterrazz/studio-station/src/setup"
)

type RunIn struct {
	// value: the name this Mac takes; empty asks it at a terminal, or keeps
	// the one it has.
	Name        string
	Interactive bool
}

var Run = call.Call[Deps, RunIn, setup.Report]{
	Name: "setup.run", Where: call.AtTarget, Applies: machine.Anywhere,
	Run: func(ctx context.Context, d Deps, in RunIn, _ call.Progress) (setup.Report, error) {
		if err := d.Setup.CheckName(in.Name); err != nil {
			return setup.Report{}, err
		}
		var ask func() (string, error)
		if d.Ask != nil {
			ask = func() (string, error) { return d.Ask.Name(ctx) }
		}
		return d.Setup.Run(ctx, in.Name, in.Interactive, ask)
	},
}
