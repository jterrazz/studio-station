package wire

import (
	"time"

	accountscalls "github.com/jterrazz/studio-station/src/accounts/calls"
	appscalls "github.com/jterrazz/studio-station/src/apps/calls"
	awakecalls "github.com/jterrazz/studio-station/src/awake/calls"
	"github.com/jterrazz/studio-station/src/core/call"
	diskcalls "github.com/jterrazz/studio-station/src/disk/calls"
	"github.com/jterrazz/studio-station/src/domain/power"
	"github.com/jterrazz/studio-station/src/domain/shortcuts"
	enginecalls "github.com/jterrazz/studio-station/src/engine/calls"
	imagescalls "github.com/jterrazz/studio-station/src/images/calls"
	machinescalls "github.com/jterrazz/studio-station/src/machines/calls"
	portscalls "github.com/jterrazz/studio-station/src/ports/calls"
	"github.com/jterrazz/studio-station/src/remote"
	settingscalls "github.com/jterrazz/studio-station/src/settings/calls"
	setupcalls "github.com/jterrazz/studio-station/src/setup/calls"
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
	setupService := setupService(env, m)
	var named setupcalls.Ask
	if env.Interactive {
		named = setupAsk{setup: setupService, ask: env.Prompt}
	}
	registry, err := call.NewRegistry(
		accountscalls.All(accountscalls.Deps{Accounts: m.accounts(), Secrets: m.secrets(), Kind: kind}),
		appscalls.All(appscalls.Deps{Apps: toolsService(env, m), Kind: kind}),
		awakecalls.All(awakecalls.Deps{Power: awakePower{m.power()}}),
		diskcalls.All(diskcalls.Deps{Storage: m.storage(env), Kind: kind}),
		enginecalls.All(enginecalls.Deps{Control: m.control(), Kind: kind}),
		imagescalls.All(imagescalls.Deps{Images: imageBuilds{m.images()}, Kind: kind}),
		machinescalls.All(machinescalls.Deps{Machines: machineCalls{m.machines(env.Interactive)}, Kind: kind}),
		portscalls.All(portscalls.Deps{Services: m.services(env), Kind: kind}),
		setupcalls.All(setupcalls.Deps{Setup: setupRun{setupService}, Ask: named}),
		settingscalls.All(settingscalls.Deps{Config: config, Ask: ask, Shortcuts: shortcuts.New(m.host), Kind: kind}),
	)
	if err != nil {
		panic(err)
	}
	return registry
}

type awakePower struct{ *power.Service }

func (awakePower) CheckFor(term time.Duration) error { return power.CheckFor(term) }
