package fake

import (
	"context"
	"encoding/json"
	"slices"

	"github.com/jterrazz/studio-station/src/core/contract"
	"github.com/jterrazz/studio-station/src/core/store"
)

type Preferences struct{ kept map[string]json.RawMessage }

var (
	_ contract.Preferences = (*Preferences)(nil)
	_ store.Migrator       = (*Preferences)(nil)
)

func NewPreferences() *Preferences { return &Preferences{kept: map[string]json.RawMessage{}} }

func (p *Preferences) groups() map[string]json.RawMessage { return p.kept }

func (p *Preferences) Read(group string, v any) error {
	raw, ok := p.groups()[group]
	if !ok {
		return nil
	}
	return json.Unmarshal(raw, v)
}

func (p *Preferences) Write(group string, v any) error {
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
