package calls

import (
	"context"

	"github.com/jterrazz/studio-station/src/apps"
	"github.com/jterrazz/studio-station/src/core/call"
	"github.com/jterrazz/studio-station/src/core/section"
)

type ListIn struct{}

var List = call.Watch[Deps, ListIn, []apps.Report]{
	Name: "tools.list", Section: section.Apps, Where: call.AtTarget, Applies: apps.Applies,
	Run: func(ctx context.Context, d Deps, _ ListIn, _ call.Progress) (<-chan call.Update[[]apps.Report], error) {
		return call.Updates(ctx, d.Apps.Watch(ctx)), nil
	},
}
