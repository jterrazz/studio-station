package wire

import (
	appscalls "github.com/jterrazz/studio-station/src/apps/calls"
	"github.com/jterrazz/studio-station/src/core/call"
	"github.com/jterrazz/studio-station/src/remote"
	"github.com/jterrazz/studio-station/src/surface/cli/kit"
)

// invariant: the one registry of this process, every family's All with its
// Deps; the Router runs its calls on this machine.
func (m machine) router(env kit.Env) *remote.Router {
	return remote.NewRouter(m.registry(env), m.profile)
}

func (m machine) registry(env kit.Env) *call.Registry {
	kind := m.profile.Kind
	registry, err := call.NewRegistry(
		appscalls.All(appscalls.Deps{Apps: toolsService(env, m), Kind: kind}),
	)
	if err != nil {
		panic(err)
	}
	return registry
}
