package wire

import (
	"time"

	accountscalls "github.com/jterrazz/studio-station/src/accounts/calls"
	appscalls "github.com/jterrazz/studio-station/src/apps/calls"
	awakecalls "github.com/jterrazz/studio-station/src/awake/calls"
	"github.com/jterrazz/studio-station/src/core/call"
	diskcalls "github.com/jterrazz/studio-station/src/disk/calls"
	"github.com/jterrazz/studio-station/src/domain/power"
	portscalls "github.com/jterrazz/studio-station/src/ports/calls"
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
		accountscalls.All(accountscalls.Deps{Accounts: m.accounts(), Secrets: m.secrets(), Kind: kind}),
		appscalls.All(appscalls.Deps{Apps: toolsService(env, m), Kind: kind}),
		awakecalls.All(awakecalls.Deps{Power: awakePower{m.power()}}),
		diskcalls.All(diskcalls.Deps{Storage: m.storage(env), Kind: kind}),
		portscalls.All(portscalls.Deps{Services: m.services(env), Kind: kind}),
		settingscalls.All(settingscalls.Deps{Config: config, Ask: ask, Kind: kind}),
	)
	if err != nil {
		panic(err)
	}
	return registry
}

type awakePower struct{ *power.Service }

func (awakePower) CheckFor(term time.Duration) error { return power.CheckFor(term) }
