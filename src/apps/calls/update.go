package calls

import (
	"context"

	"github.com/jterrazz/studio-station/src/apps"
	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/core/call"
	"github.com/jterrazz/studio-station/src/core/fail"
	"github.com/jterrazz/studio-station/src/core/section"
)

type UpdateIn struct {
	// value: no name and no method updates every Update available row.
	Names  []string
	Method []catalog.Manager
}

var Update = call.Call[Deps, UpdateIn, []apps.Outcome]{
	Name: "apps.update", Section: section.Apps, Where: call.AtTarget, Applies: apps.Applies,
	Errors: []fail.Code{Unknown, Elsewhere, NotInstalled},
	Run: func(ctx context.Context, d Deps, in UpdateIn, p call.Progress) ([]apps.Outcome, error) {
		if len(in.Names) > 0 {
			if err := d.Apps.CheckUpgrade(in.Names); err != nil {
				return nil, coded(err)
			}
		}
		step(p, "Updating "+updating(in))
		out, err := d.Apps.Upgrade(ctx, apps.UpgradeRequest{Names: in.Names, All: len(in.Names) == 0 && len(in.Method) == 0, Via: in.Method})
		return interrupted(ctx, out, err)
	},
}
