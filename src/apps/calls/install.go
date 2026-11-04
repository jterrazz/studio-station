package calls

import (
	"context"

	"github.com/jterrazz/studio-station/src/apps"
	"github.com/jterrazz/studio-station/src/core/call"
	"github.com/jterrazz/studio-station/src/core/section"
)

type InstallIn struct {
	Names []string
	// value: whether a config item the tool brings may ask its values.
	Interactive bool
}

var Install = call.Call[Deps, InstallIn, []apps.Outcome]{
	Name: "tools.install", Section: section.Apps, Where: call.AtTarget, Applies: apps.Applies,
	Run: func(ctx context.Context, d Deps, in InstallIn, _ call.Progress) ([]apps.Outcome, error) {
		if err := d.Apps.CheckInstall(ctx, in.Names); err != nil {
			return nil, err
		}
		return d.Apps.Install(ctx, apps.InstallRequest{Names: in.Names, Interactive: in.Interactive})
	},
}
