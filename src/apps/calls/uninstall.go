package calls

import (
	"context"
	"strings"

	"github.com/jterrazz/studio-station/src/apps"
	"github.com/jterrazz/studio-station/src/core/call"
	"github.com/jterrazz/studio-station/src/core/confirm"
	"github.com/jterrazz/studio-station/src/core/fail"
	"github.com/jterrazz/studio-station/src/core/section"
)

type UninstallIn struct {
	Names   []string
	Confirm string
}

var Uninstall = call.Call[Deps, UninstallIn, []apps.Outcome]{
	Name: "tools.uninstall", Section: section.Apps, Where: call.AtTarget, Applies: apps.Applies, Confirm: confirm.Remove,
	Errors: []fail.Code{confirm.YesNeeded},
	Run: func(ctx context.Context, d Deps, in UninstallIn, _ call.Progress) ([]apps.Outcome, error) {
		if err := d.Apps.CheckUninstall(in.Names); err != nil {
			return nil, err
		}
		if err := confirm.Require(confirm.For(confirm.Remove, d.Kind), strings.Join(in.Names, ", "), in.Confirm); err != nil {
			return nil, err
		}
		return d.Apps.Uninstall(ctx, in.Names)
	},
}
