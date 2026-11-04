package fake

import (
	"context"
	"encoding/json"
	"slices"
	"sync"

	"github.com/jterrazz/studio-station/src/core/contract"
	"github.com/jterrazz/studio-station/src/core/store"
)

type Preferences struct {
	// invariant: the preferences of one machine, kept in groups handed by the
	// caller (a Mac of the spec world) or its own; the lock is held while they
	// are read or written.
	kept  map[string]json.RawMessage
	owner string
	mu    sync.Locker
}

var (
	_ contract.Preferences = (*Preferences)(nil)
	_ store.Migrator       = (*Preferences)(nil)
)

func NewPreferences() *Preferences {
	return PreferencesOver(map[string]json.RawMessage{}, "this machine", &sync.Mutex{})
}

func PreferencesOver(groups map[string]json.RawMessage, owner string, mu sync.Locker) *Preferences {
	return &Preferences{kept: groups, owner: owner, mu: mu}
}

func (p *Preferences) groups() map[string]json.RawMessage { return p.kept }

func (p *Preferences) lock() func() {
	p.mu.Lock()
	return p.mu.Unlock
}

func (p *Preferences) File() string {
	// value: where the preferences are kept, as a page names it.
	return "the preferences of " + p.owner
}

func (p *Preferences) Read(group string, v any) error {
	defer p.lock()()
	raw, ok := p.groups()[group]
	if !ok {
		return nil
	}
	return json.Unmarshal(raw, v)
}

func (p *Preferences) Write(group string, v any) error {
	defer p.lock()()
	raw, err := json.Marshal(v)
	if err != nil {
		return err
	}
	if slices.Contains([]string{"null", `""`, "0", "false", "{}", "[]"}, string(raw)) {
		delete(p.groups(), group)
		return nil
	}
	p.groups()[group] = raw
	return nil
}

func (p *Preferences) Migrate(context.Context) error { return nil }
