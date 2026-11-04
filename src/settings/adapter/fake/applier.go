package fake

import (
	"context"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/settings"
	"github.com/jterrazz/studio-station/src/settings/contract"
	"github.com/jterrazz/studio-station/src/world"
)

type Applier struct {
	w    *world.World
	mac  string
	kind catalog.EffectKind
}

var _ contract.Applier = (*Applier)(nil)

const Ours = "station's"

func ApplierOf(w *world.World, mac string, kind catalog.EffectKind) *Applier {
	return &Applier{w: w, mac: mac, kind: kind}
}

func NewApplier(kind catalog.EffectKind) *Applier {
	w := &world.World{Here: "here"}
	w.Mac("here")
	return ApplierOf(w, "here", kind)
}

func (a *Applier) Kind() catalog.EffectKind { return a.kind }

func (a *Applier) held() map[string]string {
	m := a.w.Mac(a.mac)
	if m.Settings == nil {
		m.Settings = map[string]string{}
	}
	return m.Settings
}

func (a *Applier) Edit(name, text string) {
	// value: the owner writes their own text in place of a setting's.
	a.w.Lock()
	defer a.w.Unlock()
	a.held()[name] = text
}

func (a *Applier) Observe(_ context.Context, it catalog.ConfigItem) (settings.Observation, error) {
	a.w.Lock()
	defer a.w.Unlock()
	text, ok := a.held()[it.Name]
	if !ok {
		return settings.Observation{State: catalog.StateAbsent, After: []settings.KeyValue{{Key: it.Name, Value: Ours}}}, nil
	}
	o := settings.Observation{State: catalog.StateOK, Digest: text}
	if text != Ours {
		o.State, o.Detail = catalog.StateDegraded, "differs from station's"
		o.Before, o.After = []settings.KeyValue{{Key: it.Name, Value: text}}, []settings.KeyValue{{Key: it.Name, Value: Ours}}
	}
	return o, nil
}

func (a *Applier) Apply(_ context.Context, it catalog.ConfigItem, _ map[string]string) (settings.Change, error) {
	a.w.Lock()
	defer a.w.Unlock()
	a.held()[it.Name] = Ours
	return settings.Change{Steps: []settings.ChangeStep{{Label: it.Name, State: catalog.StateOK, Detail: "applied"}}}, nil
}

func (a *Applier) Undo(_ context.Context, it catalog.ConfigItem) (settings.Change, error) {
	a.w.Lock()
	defer a.w.Unlock()
	if a.held()[it.Name] != Ours {
		return settings.Change{Steps: []settings.ChangeStep{{Label: it.Name, State: catalog.StateOK, Detail: "yours, kept"}}}, nil
	}
	delete(a.held(), it.Name)
	return settings.Change{Steps: []settings.ChangeStep{{Label: it.Name, State: catalog.StateOK, Detail: "undone"}}}, nil
}
