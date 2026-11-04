package calls

import (
	"context"

	"github.com/jterrazz/studio-station/src/background"
	"github.com/jterrazz/studio-station/src/core/call"
	"github.com/jterrazz/studio-station/src/core/section"
)

type ListIn struct{}

var List = call.Call[Deps, ListIn, []background.Task]{
	Name: "background.list", Section: section.Background, Where: call.AtTarget, Applies: background.Applies,
	Run: func(ctx context.Context, d Deps, _ ListIn, _ call.Progress) ([]background.Task, error) {
		return tasks(ctx, d)
	},
}
