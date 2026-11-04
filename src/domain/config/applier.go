package config

import (
	"context"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/settings"
	settingscontract "github.com/jterrazz/studio-station/src/settings/contract"
)

// value: a procedure's check, install and uninstall as an Applier: its
// check is what Observe reads; a procedure with no check is run once and
// cannot be read back.
type procedureApplier struct {
	p catalog.Procedure
	h handler
}

func (a procedureApplier) Kind() catalog.EffectKind { return a.p.Kind() }

func (a procedureApplier) Observe(ctx context.Context, _ catalog.ConfigItem) (settings.Observation, error) {
	if a.h.check == nil {
		return settings.Observation{State: catalog.StateAbsent, Detail: "run once", Blind: true}, nil
	}
	state, detail := a.h.check(ctx)
	return settings.Observation{State: state, Detail: detail}, nil
}

func (a procedureApplier) Apply(ctx context.Context, _ catalog.ConfigItem, values map[string]string) (settings.Change, error) {
	return a.h.install(ctx, values)
}

func (a procedureApplier) Undo(ctx context.Context, _ catalog.ConfigItem) (settings.Change, error) {
	if a.h.uninstall == nil {
		return settings.Change{}, settingscontract.ErrNoUndo
	}
	return a.h.uninstall(ctx)
}

// value: the Appliers this service runs its items through: the ones it was
// given, and its own procedures.
func (s *Service) Appliers() settingscontract.Appliers { return s.appliers }
