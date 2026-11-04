package calls

import (
	"context"
	"strings"

	"github.com/jterrazz/studio-station/src/core/call"
	"github.com/jterrazz/studio-station/src/core/confirm"
	"github.com/jterrazz/studio-station/src/core/fail"
	"github.com/jterrazz/studio-station/src/core/section"
	"github.com/jterrazz/studio-station/src/settings"
)

type UninstallIn struct {
	Names   []string
	Confirm string
}

var Uninstall = call.Call[Deps, UninstallIn, []settings.Change]{
	Name: "config.uninstall", Section: section.Settings, Where: call.AtTarget, Applies: settings.Applies, Confirm: confirm.OffHere,
	Errors: []fail.Code{confirm.YesNeeded},
	Run: func(ctx context.Context, d Deps, in UninstallIn, _ call.Progress) ([]settings.Change, error) {
		if err := d.Config.CheckUninstall(in.Names); err != nil {
			return nil, err
		}
		if err := confirm.Require(confirm.For(confirm.OffHere, d.Kind), strings.Join(in.Names, ", "), in.Confirm); err != nil {
			return nil, err
		}
		items, err := d.Config.Find(in.Names)
		if err != nil {
			return nil, err
		}
		changes := make([]settings.Change, 0, len(items))
		for _, it := range items {
			change, _ := d.Config.Uninstall(ctx, it.Name)
			changes = append(changes, change)
		}
		return changes, nil
	},
}
