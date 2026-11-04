package wire

import (
	appscalls "github.com/jterrazz/studio-station/src/apps/calls"
	"github.com/jterrazz/studio-station/src/core/call"
	diskcalls "github.com/jterrazz/studio-station/src/disk/calls"
	"github.com/jterrazz/studio-station/src/remote"
	settingscalls "github.com/jterrazz/studio-station/src/settings/calls"
	"github.com/jterrazz/studio-station/src/surface/cli/kit"
)

// invariant: the one registry of this process, every family's All with its
// Deps; the Router runs its calls on this machine.
func (m machine) router(env kit.Env) *remote.Router {
	return remote.NewRouter(m.registry(env), m.profile)
}

func (m machine) registry(env kit.Env) *call.Registry {
	kind := m.profile.Kind
	config := configService(m)
	var ask settingscalls.Ask
	if env.Interactive {
		ask = configAsk{config: config, ask: env.Prompt}
	}
	registry, err := call.NewRegistry(
		appscalls.All(appscalls.Deps{Apps: toolsService(env, m), Kind: kind}),
		diskcalls.All(diskcalls.Deps{Storage: m.storage(env), Kind: kind}),
		settingscalls.All(settingscalls.Deps{Config: config, Ask: ask, Kind: kind}),
	)
	if err != nil {
		panic(err)
	}
	return registry
}
