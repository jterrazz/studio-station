package calls

import (
	"context"

	"github.com/jterrazz/studio-station/src/core/call"
	"github.com/jterrazz/studio-station/src/core/section"
	"github.com/jterrazz/studio-station/src/settings"
)

type InstallIn struct {
	Names []string
	// value: the values given for an item, by its name; an item with inputs
	// and none given asks them, at a terminal.
	Values      map[string]map[string]string
	Interactive bool
}

var Install = call.Call[Deps, InstallIn, []settings.Change]{
	Name: "config.install", Section: section.Settings, Where: call.AtTarget, Applies: settings.Applies,
	Run: func(ctx context.Context, d Deps, in InstallIn, _ call.Progress) ([]settings.Change, error) {
		// invariant: every item is judged before any question and any
		// effect; an item that fails is a broken change, the others still run.
		if err := d.Config.CheckInstall(in.Names, in.Interactive); err != nil {
			return nil, err
		}
		items, err := d.Config.Find(in.Names)
		if err != nil {
			return nil, err
		}
		changes := make([]settings.Change, 0, len(items))
		for _, it := range items {
			values, given := in.Values[it.Name]
			if !given && len(it.Inputs) > 0 && in.Interactive && d.Ask != nil {
				if values, err = d.Ask.Values(it); err != nil {
					return nil, err
				}
			}
			change, _ := d.Config.Install(ctx, it.Name, values, in.Interactive)
			changes = append(changes, change)
		}
		return changes, nil
	},
}
