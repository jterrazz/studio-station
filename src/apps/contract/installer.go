package contract

import (
	"context"
	"errors"

	"github.com/jterrazz/studio-station/src/apps"
	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/core/call"
)

var (
	ErrMissing = errors.New("its manager is not installed")
	// invariant: a method that keeps no list of what it installed: an app's
	// own probe, or its bundle, tells whether it is there.
	ErrUnlisted = errors.New("its method keeps no list")
	// invariant: a verb its Abilities deny is refused, never faked.
	ErrCannot = errors.New("its method cannot do this")
)

type Installer interface {
	Method() catalog.Method
	Manager() Manager
	Can() Abilities
	Ready(ctx context.Context) error
	Inventory(ctx context.Context) (map[string]string, error)
	Outdated(ctx context.Context, keys []string) (map[string]apps.Currency, error)
	Install(ctx context.Context, installs []catalog.Install, p call.Progress) error
	Update(ctx context.Context, installs []catalog.Install, p call.Progress) error
	Remove(ctx context.Context, installs []catalog.Install, p call.Progress) error
}

type Manager struct {
	// value: the manager's word in a row's detail: "brew did not answer".
	Name string
	// value: the catalogue app that provides the manager, named when it is missing.
	App string
	// value: the catalogue app installed before any install of this method.
	Needs string
}

type Abilities struct {
	Install bool
	Update  bool
	Remove  bool
	// invariant: a bare update reaches the method; one it does not reach is
	// updated by name only.
	UpdateAll bool
}

type Installers map[catalog.Method]Installer // invariant: the one registry; no other code switches on a method.

type Lined interface {
	// invariant: a method whose installs and updates ride one shared run (D7's
	// brew bundle) spells each install's line; the Bundler runs the lines of
	// every such method at once.
	Line(i catalog.Install) string
}

type Bundler interface {
	Bundle(ctx context.Context, lines []string, update bool, p call.Progress) error
}
