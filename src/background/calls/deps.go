package calls

import (
	"context"

	"github.com/jterrazz/studio-station/src/background"
	"github.com/jterrazz/studio-station/src/background/contract"
	"github.com/jterrazz/studio-station/src/core/call"
	"github.com/jterrazz/studio-station/src/core/machine"
)

type Deps struct {
	Jobs contract.Jobs
	// value: the owner's home and the system root, so a path reads as the
	// owner types it.
	Home, Root string
	Kind       machine.Kind
}

func All(d Deps) []call.Handler {
	return []call.Handler{call.Bind(List, d)}
}

func tasks(ctx context.Context, d Deps) ([]background.Task, error) {
	jobs, err := d.Jobs.List(ctx)
	if err != nil {
		return nil, err
	}
	return background.Judge(jobs, d.Root, d.Home), nil
}
