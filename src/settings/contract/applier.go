package contract

import (
	"context"
	"errors"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/settings"
)

// value: the Undo of an effect station cannot take back (a login, a key):
// off leaves it as it is.
var ErrNoUndo = errors.New("station does not undo it")

type Applier interface {
	// value: the kind of effect it writes, its key in Appliers.
	Kind() catalog.EffectKind
	// invariant: Observe reads and never writes; a setting it cannot read
	// back reads Blind.
	Observe(ctx context.Context, it catalog.ConfigItem) (settings.Observation, error)
	// invariant: once Apply returns no error, Observe reads it present and
	// exactly as station writes it.
	Apply(ctx context.Context, it catalog.ConfigItem, values map[string]string) (settings.Change, error)
	// invariant: Undo removes only what station wrote; once it returns no
	// error, Observe reads it absent. ErrNoUndo changes nothing.
	Undo(ctx context.Context, it catalog.ConfigItem) (settings.Change, error)
}

// value: the strategy per effect: the one Applier of each kind.
type Appliers map[catalog.EffectKind]Applier

func NewAppliers(all ...Applier) Appliers {
	out := Appliers{}
	for _, a := range all {
		out[a.Kind()] = a
	}
	return out
}

func (a Appliers) For(it catalog.ConfigItem) (Applier, bool) {
	if it.Effect == nil {
		return nil, false
	}
	applier, ok := a[it.Effect.Kind()]
	return applier, ok
}
