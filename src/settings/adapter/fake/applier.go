package fake

import (
	"context"
	"sync"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/settings"
	"github.com/jterrazz/studio-station/src/settings/contract"
)

// value: an Applier of one kind over memory: what each setting holds now,
// station's text or the owner's.
type Applier struct {
	mu   sync.Mutex
	kind catalog.EffectKind
	held map[string]string
}

var _ contract.Applier = (*Applier)(nil)

const ours = "station's"

func NewApplier(kind catalog.EffectKind) *Applier {
	return &Applier{kind: kind, held: map[string]string{}}
}

func (a *Applier) Kind() catalog.EffectKind { return a.kind }

// value: the owner writes their own text in place of a setting's.
func (a *Applier) Edit(name, text string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.held[name] = text
}

func (a *Applier) Observe(_ context.Context, it catalog.ConfigItem) (settings.Observation, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	text, ok := a.held[it.Name]
	if !ok {
		return settings.Observation{State: catalog.StateAbsent, Where: it.Name}, nil
	}
	o := settings.Observation{State: catalog.StateOK, Digest: text, Where: it.Name}
	if text != ours {
		o.State, o.Diff = catalog.StateDegraded, settings.UnifiedDiff(ours+"\n", text+"\n", "station", it.Name)
	}
	return o, nil
}

func (a *Applier) Apply(_ context.Context, it catalog.ConfigItem, _ map[string]string) (settings.Change, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.held[it.Name] = ours
	return settings.Change{Steps: []settings.ChangeStep{{Label: it.Name, State: catalog.StateOK, Detail: "written"}}}, nil
}

func (a *Applier) Undo(_ context.Context, it catalog.ConfigItem) (settings.Change, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.held[it.Name] == ours {
		delete(a.held, it.Name)
	}
	return settings.Change{Steps: []settings.ChangeStep{{Label: it.Name, State: catalog.StateOK, Detail: "removed"}}}, nil
}
