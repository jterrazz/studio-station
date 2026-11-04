package calls

import (
	"context"

	"github.com/jterrazz/studio-station/src/core/call"
	"github.com/jterrazz/studio-station/src/core/machine"
	"github.com/jterrazz/studio-station/src/engine"
)

type Deps struct {
	Control Control
	Kind    machine.Kind
}

type Control interface {
	// value: refuses a verb off a host, or before the engine's job is installed.
	Check(verb string) error
	Plan(ctx context.Context) (engine.Work, error)
	Stop(ctx context.Context, w engine.Work) (engine.StopReport, error)
	Start(ctx context.Context) (engine.StartReport, error)
}

func All(d Deps) []call.Handler {
	return []call.Handler{call.Bind(Start, d), call.Bind(Stop, d)}
}
