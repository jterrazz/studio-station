package calls

import (
	"context"

	"github.com/jterrazz/studio-station/src/core/call"
	"github.com/jterrazz/studio-station/src/core/confirm"
	"github.com/jterrazz/studio-station/src/core/fail"
	"github.com/jterrazz/studio-station/src/core/machine"
	"github.com/jterrazz/studio-station/src/engine"
)

type StartIn struct{}

var Start = call.Call[Deps, StartIn, engine.StartReport]{
	Name: "engine.start", Where: call.InProcess, Applies: machine.Anywhere,
	Run: func(ctx context.Context, d Deps, _ StartIn, _ call.Progress) (engine.StartReport, error) {
		if err := d.Control.Check("start"); err != nil {
			return engine.StartReport{}, err
		}
		return d.Control.Start(ctx)
	},
}

const Deleted = engine.ResultDeleted

type StopIn struct{ Confirm string }

type StopOut struct {
	// value: what the stop ends; handed back with the confirmation it asks,
	// so the question names it.
	Work engine.Work
	// value: set once the stop ran, whole or part-way.
	Report *engine.StopReport
}

var Stop = call.Call[Deps, StopIn, StopOut]{
	Name: "engine.stop", Where: call.InProcess, Applies: machine.Anywhere, Confirm: confirm.StopEngine,
	Errors: []fail.Code{confirm.YesNeeded},
	Run: func(ctx context.Context, d Deps, in StopIn, _ call.Progress) (StopOut, error) {
		// invariant: an engine running no VM stops without a question; one
		// running VMs asks first, since the temporary ones are deleted.
		if err := d.Control.Check("stop"); err != nil {
			return StopOut{}, err
		}
		work, err := d.Control.Plan(ctx)
		if err != nil {
			return StopOut{}, err
		}
		if !work.Empty() {
			if refused := confirm.Require(confirm.For(confirm.StopEngine, d.Kind), "stop", in.Confirm); refused != nil {
				return StopOut{Work: work}, refused
			}
		}
		report, err := d.Control.Stop(ctx, work)
		return StopOut{Work: work, Report: &report}, err
	},
}
