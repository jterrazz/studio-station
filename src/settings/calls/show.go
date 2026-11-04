package calls

import (
	"context"

	"github.com/jterrazz/studio-station/src/core/call"
	"github.com/jterrazz/studio-station/src/core/fail"
	"github.com/jterrazz/studio-station/src/core/section"
	"github.com/jterrazz/studio-station/src/settings"
)

type ShowIn struct{ Name string }

var Show = call.Call[Deps, ShowIn, settings.Row]{
	Name: "settings.show", Section: section.Settings, Where: call.AtTarget, Applies: settings.Applies,
	Errors: []fail.Code{Unknown},
	Run: func(ctx context.Context, d Deps, in ShowIn, _ call.Progress) (settings.Row, error) {
		it, err := find(d, in.Name, false)
		if err != nil {
			return settings.Row{}, err
		}
		row, _ := judge(ctx, d, it, offHere(d))
		return row, nil
	},
}
