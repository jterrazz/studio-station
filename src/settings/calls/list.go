package calls

import (
	"context"

	"github.com/jterrazz/studio-station/src/core/call"
	"github.com/jterrazz/studio-station/src/core/section"
	"github.com/jterrazz/studio-station/src/settings"
)

type ListIn struct{}

var List = call.Watch[Deps, ListIn, []settings.Report]{
	Name: "config.list", Section: section.Settings, Where: call.AtTarget, Applies: settings.Applies,
	Run: func(ctx context.Context, d Deps, _ ListIn, _ call.Progress) (<-chan call.Update[[]settings.Report], error) {
		reports, err := d.Config.Watch(ctx)
		if err != nil {
			return nil, err
		}
		return call.Updates(ctx, reports), nil
	},
}
