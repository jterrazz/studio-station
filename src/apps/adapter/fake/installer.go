package fake

import (
	"context"
	"errors"
	"maps"
	"slices"
	"sync"

	"github.com/jterrazz/studio-station/src/apps"
	"github.com/jterrazz/studio-station/src/apps/contract"
	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/core/call"
)

var errScripted = errors.New("the run failed, as the world scripted it")

type State struct {
	Absent    bool
	Installed map[string]string
	Behind    []string
	Fails     bool
}

type Installer struct {
	method catalog.Method
	mu     sync.Mutex
	state  *State
}

func New(m catalog.Method, s *State) *Installer {
	if s.Installed == nil {
		s.Installed = map[string]string{}
	}
	return &Installer{method: m, state: s}
}

func (f *Installer) Method() catalog.Method { return f.method }

func (f *Installer) Manager() contract.Manager {
	return contract.Manager{Name: string(f.method), App: string(f.method)}
}

func (f *Installer) Can() contract.Abilities {
	return contract.Abilities{Install: true, Update: true, Remove: true, UpdateAll: true}
}

func (f *Installer) Ready(context.Context) error {
	if f.state.Absent {
		return contract.ErrMissing
	}
	return nil
}

func (f *Installer) Inventory(ctx context.Context) (map[string]string, error) {
	if err := f.Ready(ctx); err != nil {
		return nil, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	return maps.Clone(f.state.Installed), nil
}

func (f *Installer) Outdated(_ context.Context, keys []string) (map[string]apps.Currency, error) {
	out := map[string]apps.Currency{}
	for _, k := range keys {
		out[k] = apps.Current
		if slices.Contains(f.state.Behind, k) {
			out[k] = apps.Behind
		}
	}
	return out, nil
}

func (f *Installer) run(ctx context.Context, installs []catalog.Install, apply func(key string)) error {
	if err := f.Ready(ctx); err != nil {
		return err
	}
	if f.state.Fails {
		return errScripted
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, i := range installs {
		apply(i.Key())
	}
	return nil
}

func (f *Installer) Install(ctx context.Context, installs []catalog.Install, _ call.Progress) error {
	return f.run(ctx, installs, func(k string) { f.state.Installed[k] = "1.0.0" })
}

func (f *Installer) Update(ctx context.Context, installs []catalog.Install, _ call.Progress) error {
	return f.run(ctx, installs, func(k string) {
		f.state.Behind = slices.DeleteFunc(f.state.Behind, func(b string) bool { return b == k })
	})
}

func (f *Installer) Remove(ctx context.Context, installs []catalog.Install, _ call.Progress) error {
	return f.run(ctx, installs, func(k string) { delete(f.state.Installed, k) })
}
