package calls

import (
	"context"

	"github.com/jterrazz/studio-station/src/apps"
	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/core/call"
	"github.com/jterrazz/studio-station/src/core/section"
)

type UpgradeIn struct {
	// value: no name and no manager upgrades every installed tool.
	Names []string
	Via   []catalog.Manager
}

var Upgrade = call.Call[Deps, UpgradeIn, []apps.Outcome]{
	Name: "tools.upgrade", Section: section.Apps, Where: call.AtTarget, Applies: apps.Applies,
	Run: func(ctx context.Context, d Deps, in UpgradeIn, _ call.Progress) ([]apps.Outcome, error) {
		if len(in.Names) > 0 {
			if err := d.Apps.CheckUpgrade(in.Names); err != nil {
				return nil, err
			}
		}
		return d.Apps.Upgrade(ctx, apps.UpgradeRequest{Names: in.Names, All: len(in.Names) == 0 && len(in.Via) == 0, Via: in.Via})
	},
}
