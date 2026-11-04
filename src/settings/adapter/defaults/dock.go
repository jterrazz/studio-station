package defaults

import (
	"context"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/platform/local"
	"github.com/jterrazz/studio-station/src/settings"
	"github.com/jterrazz/studio-station/src/settings/contract"
)

const dockDomain = "com.apple.dock"

type Dock struct {
	run local.Runner
	of  catalog.Procedure
}

var _ contract.Applier = Dock{}

func NewDock(run local.Runner) []contract.Applier {
	return []contract.Applier{Dock{run: run, of: catalog.ProcDockReset}, Dock{run: run, of: catalog.ProcDockSpacer}}
}

func (d Dock) Kind() catalog.EffectKind { return d.of.Kind() }

func (d Dock) Observe(_ context.Context, it catalog.ConfigItem) (settings.Observation, error) {
	return settings.Observation{
		State: catalog.StateAbsent, Detail: "run once", Blind: true,
		Before: []settings.KeyValue{{Key: "Dock", Value: "not read back"}},
		After:  settings.Sets(it),
	}, nil
}

func (d Dock) Apply(ctx context.Context, it catalog.ConfigItem, _ map[string]string) (settings.Change, error) {
	// outside: `defaults delete` fails when the domain is already absent, which is the
	// state the reset asks for.
	if d.of == catalog.ProcDockReset {
		_, _ = local.Probe(ctx, d.run, local.Command("defaults", "delete", dockDomain))
	} else if _, err := local.Must(ctx, d.run, local.Command("defaults", "write", dockDomain, "persistent-apps", "-array-add", `{"tile-type"="small-spacer-tile";}`)); err != nil {
		return settings.Change{}, err
	}
	if _, err := local.Must(ctx, d.run, local.Command("killall", "Dock")); err != nil {
		return settings.Change{}, err
	}
	var change settings.Change
	for _, kv := range settings.Sets(it) {
		change.Steps = append(change.Steps, settings.ChangeStep{Label: "dock", State: catalog.StateOK, Detail: kv.Value})
	}
	return change, nil
}

func (Dock) Undo(context.Context, catalog.ConfigItem) (settings.Change, error) {
	return settings.Change{}, contract.ErrNoUndo
}
