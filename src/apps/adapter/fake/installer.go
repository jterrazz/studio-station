package fake

import (
	"context"
	"errors"
	"maps"
	"slices"
	"strings"

	"github.com/jterrazz/studio-station/src/apps"
	"github.com/jterrazz/studio-station/src/apps/contract"
	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/core/call"
	"github.com/jterrazz/studio-station/src/world"
)

var errScripted = errors.New("the run failed, as the world scripted it")

const here = "here"

type State struct {
	Absent    bool
	Installed map[string]string
	Behind    []string
	Fails     bool
}

type Installer struct {
	// invariant: one method's installer on one Mac of the world: what it
	// holds is that Mac's slice, and a fault scripted for it fails its run.
	w      *world.World
	mac    string
	method catalog.Method
}

type LinedInstaller struct {
	// invariant: an installer of a method whose installs ride the one
	// bundle, as D7's brew bundle carries brew, cask, mas, npm and uv.
	*Installer
}

func InstallerOf(w *world.World, mac string, m catalog.Method, lined bool) contract.Installer {
	i := &Installer{w: w, mac: mac, method: m}
	if lined {
		return LinedInstaller{i}
	}
	return i
}

func New(m catalog.Method, s *State) *Installer {
	w := &world.World{Here: here}
	w.Mac(here).Apps = map[catalog.Method]*world.Manager{m: {Absent: s.Absent, Installed: maps.Clone(s.Installed), Behind: slices.Clone(s.Behind)}}
	if s.Fails {
		w.Script = []world.Fault{{Call: "Installer.Install"}, {Call: "Installer.Update"}, {Call: "Installer.Remove"}}
	}
	return &Installer{w: w, mac: here, method: m}
}

func (f *Installer) manager() *world.Manager { return manager(f.w, f.mac, f.method) }

func manager(w *world.World, mac string, m catalog.Method) *world.Manager {
	slice := w.Mac(mac)
	if slice.Apps == nil {
		slice.Apps = map[catalog.Method]*world.Manager{}
	}
	s := slice.Apps[m]
	if s == nil {
		s = &world.Manager{}
		slice.Apps[m] = s
	}
	if s.Installed == nil {
		s.Installed = map[string]string{}
	}
	return s
}

func (f *Installer) Method() catalog.Method { return f.method }

func (f *Installer) Manager() contract.Manager {
	return contract.Manager{Name: string(f.method), App: string(f.method)}
}

func (f *Installer) Can() contract.Abilities {
	return contract.Abilities{Install: true, Update: true, Remove: true, UpdateAll: true}
}

func (f *Installer) Ready(context.Context) error {
	f.w.Lock()
	defer f.w.Unlock()
	if f.manager().Absent {
		return contract.ErrMissing
	}
	return nil
}

func (f *Installer) Inventory(ctx context.Context) (map[string]string, error) {
	if err := f.Ready(ctx); err != nil {
		return nil, err
	}
	f.w.Lock()
	defer f.w.Unlock()
	return maps.Clone(f.manager().Installed), nil
}

func (f *Installer) Outdated(_ context.Context, keys []string) (map[string]apps.Currency, error) {
	f.w.Lock()
	defer f.w.Unlock()
	out := map[string]apps.Currency{}
	for _, k := range keys {
		out[k] = apps.Current
		if slices.Contains(f.manager().Behind, k) {
			out[k] = apps.Behind
		}
	}
	return out, nil
}

func (f *Installer) run(ctx context.Context, verb string, installs []catalog.Install, apply func(s *world.Manager, key string)) error {
	if err := f.Ready(ctx); err != nil {
		return err
	}
	f.w.Lock()
	defer f.w.Unlock()
	if f.w.Fails("Installer." + verb) {
		return errScripted
	}
	for _, i := range installs {
		apply(f.manager(), i.Key())
	}
	return nil
}

func installed(s *world.Manager, k string) { s.Installed[k] = "1.0.0" }

func updated(s *world.Manager, k string) {
	s.Behind = slices.DeleteFunc(s.Behind, func(b string) bool { return b == k })
}

func (f *Installer) Install(ctx context.Context, installs []catalog.Install, _ call.Progress) error {
	return f.run(ctx, "Install", installs, installed)
}

func (f *Installer) Update(ctx context.Context, installs []catalog.Install, _ call.Progress) error {
	return f.run(ctx, "Update", installs, updated)
}

func (f *Installer) Remove(ctx context.Context, installs []catalog.Install, _ call.Progress) error {
	return f.run(ctx, "Remove", installs, func(s *world.Manager, k string) { delete(s.Installed, k) })
}

func (l LinedInstaller) Line(i catalog.Install) string { return string(l.method) + " " + i.Key() }

type Bundler struct {
	// invariant: the one bundle of a Mac of the world: each line installs, or
	// updates, its key under its method; a fault scripted for it fails the
	// whole run and changes nothing.
	w   *world.World
	mac string
}

var (
	_ contract.Bundler = (*Bundler)(nil)
	_ contract.Lined   = LinedInstaller{}
)

func BundlerOf(w *world.World, mac string) *Bundler { return &Bundler{w: w, mac: mac} }

func (b *Bundler) Name() string { return "the bundle" }

func (b *Bundler) Bundle(_ context.Context, lines []string, update bool, _ call.Progress) error {
	b.w.Lock()
	defer b.w.Unlock()
	if b.w.Fails("Bundler.Bundle") {
		return errScripted
	}
	apply := installed
	if update {
		apply = updated
	}
	for _, line := range lines {
		method, key, ok := strings.Cut(line, " ")
		if !ok {
			return errors.New("the bundle cannot read the line " + line)
		}
		s := manager(b.w, b.mac, catalog.Method(method))
		if s.Absent {
			return contract.ErrMissing
		}
		apply(s, key)
	}
	return nil
}
