package wire

import (
	"context"
	"time"

	"github.com/jterrazz/studio-station/src/activity"
	"github.com/jterrazz/studio-station/src/engine/process"
	"github.com/jterrazz/studio-station/src/settings/adapter/docker"
	"github.com/jterrazz/studio-station/src/settings/adapter/gitaliases"

	accountscalls "github.com/jterrazz/studio-station/src/accounts/calls"
	activitycalls "github.com/jterrazz/studio-station/src/activity/calls"
	activitycontract "github.com/jterrazz/studio-station/src/activity/contract"
	appscalls "github.com/jterrazz/studio-station/src/apps/calls"
	awakecalls "github.com/jterrazz/studio-station/src/awake/calls"
	backgroundcalls "github.com/jterrazz/studio-station/src/background/calls"
	"github.com/jterrazz/studio-station/src/core/call"
	coremachine "github.com/jterrazz/studio-station/src/core/machine"
	diskcalls "github.com/jterrazz/studio-station/src/disk/calls"
	"github.com/jterrazz/studio-station/src/domain/power"
	enginecalls "github.com/jterrazz/studio-station/src/engine/calls"
	imagescalls "github.com/jterrazz/studio-station/src/images/calls"
	machinescalls "github.com/jterrazz/studio-station/src/machines/calls"
	overviewcalls "github.com/jterrazz/studio-station/src/overview/calls"
	portscalls "github.com/jterrazz/studio-station/src/ports/calls"
	settingscalls "github.com/jterrazz/studio-station/src/settings/calls"
	settingscontract "github.com/jterrazz/studio-station/src/settings/contract"
	setupcalls "github.com/jterrazz/studio-station/src/setup/calls"
	"github.com/jterrazz/studio-station/src/surface/cli/kit"
)

// invariant: the one registry of this process, every family's All with its
// Deps; the Router runs its calls on this machine.
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
	sections := statusDeps(env, m)
	here := &localCalls{profile: m.profile, late: m.far}
	sections.Calls, sections.Kind = here, kind
	registry, err := call.NewRegistry(
		activitycalls.All(m.activityDeps(sections.Meter)),
		overviewcalls.All(sections),
		accountscalls.All(accountscalls.Deps{Accounts: m.accounts(), Shares: m.secrets(), Kind: kind}),
		appscalls.All(appscalls.Deps{Apps: toolsService(env, m), Kind: kind}),
		awakecalls.All(m.awakeDeps()),
		backgroundcalls.All(m.backgroundDeps()),
		diskcalls.All(diskcalls.Deps{Disk: m.storage(env), Kind: kind}),
		enginecalls.All(enginecalls.Deps{Control: m.control(), Kind: kind}),
		imagescalls.All(imagescalls.Deps{Images: imageBuilds{m.images()}, Kind: kind}),
		machinescalls.All(machinescalls.Deps{Machines: machineCalls{Service: m.machines(env.Interactive), m: m}, Kind: kind, Laptop: m.profile.Caps.Battery}),
		portscalls.All(portscalls.Deps{Ports: m.services(env), Tailnet: m.tailnetStatus(), Kind: kind,
			Declarations: m.stores.declarations, Routes: m.routes(), Far: farPorts{m}}),
		setupcalls.All(setupcalls.Deps{Setup: setupRun{setupService}, Ask: named}),
		settingscalls.All(settingscalls.Deps{
			Settings: config, Appliers: config.Appliers(), Applied: m.stores.applied, OffHere: settingscontract.OffHere(m.stores.prefs),
			Now: m.host.Clock.Now, Ask: ask, Shortcuts: docker.New(m.host.Run), GitAliases: gitaliases.NewShortcuts(m.host.Run), Shell: m.shell(), Kind: kind,
			Facts: m.facts(), Caps: m.profile.Caps, Admin: m.host.Env.Euid() == 0, Mac: m.profile.Ref.Name, Fleet: settingsAllMachines{m}, Public: settingsPublic{here},
		}),
	)
	if err != nil {
		panic(err)
	}
	here.registry = registry
	return registry
}

// value: the Router a call reads this machine's other calls through (the
// Overview its sections): this process's own once it is built, so a call
// of the engine reaches the engine; else the registry, past the same gate
// as a frame.
type localCalls struct {
	registry *call.Registry
	profile  coremachine.Profile
	late     *late
}

func (l *localCalls) routed() call.Router {
	if l.late == nil || l.late.router == nil {
		return nil
	}
	return l.late.router
}

func (l *localCalls) Invoke(ctx context.Context, _ coremachine.Ref, name string, in, out any, p call.Progress) error {
	if r := l.routed(); r != nil {
		return r.Invoke(ctx, coremachine.Ref{}, name, in, out, p)
	}
	v, err := l.registry.Run(ctx, l.profile, name, in, p)
	call.Set(out, v)
	return err
}

func (l *localCalls) Watch(ctx context.Context, _ coremachine.Ref, name string, in any, p call.Progress) (<-chan call.Update[any], error) {
	if r := l.routed(); r != nil {
		return r.Watch(ctx, coremachine.Ref{}, name, in, p)
	}
	return l.registry.Stream(ctx, l.profile, name, in, p)
}

type awakePower struct{ *power.Service }

// invariant: the history a Mac's engine keeps, asked over its socket.
func engineKept(api engineAPI) func(ctx context.Context, name string) (activity.Kept, time.Time, bool, error) {
	return func(ctx context.Context, name string) (activity.Kept, time.Time, bool, error) {
		return keptOf(ctx, api, name)
	}
}

// invariant: the process that holds a VM's guest, as this Mac's engine names it.
func engineHolder(api engineAPI) func(ctx context.Context, vm string) (int, error) {
	return func(ctx context.Context, vm string) (int, error) {
		views, err := api.VMs(ctx)
		if err != nil {
			return 0, err
		}
		for _, v := range views {
			if v.Name == vm {
				return v.Holder, nil
			}
		}
		return 0, nil
	}
}

func keptOf(ctx context.Context, api engineAPI, name string) (activity.Kept, time.Time, bool, error) {
	var out process.ActivityOut
	if err := api.do(ctx, process.Activity, nil, &out); err != nil {
		return activity.Kept{}, time.Time{}, false, err
	}
	for _, kept := range out.Machines {
		if kept.Machine == name {
			return activity.Kept{Last: kept.Last, Series: activity.SeriesOf(kept.Fine, kept.Coarse)}, out.Since, true, nil
		}
	}
	return activity.Kept{}, out.Since, false, nil
}

// value: a sample composes its free space from the Disk family, and on a
// Mac its thermal and battery from Power: a VM's are its Mac's.
func (m machine) activityDeps(meter activitycontract.Meter) activitycalls.Deps {
	d := activitycalls.Deps{Meter: meter, Volume: m.disk(), Name: m.profile.Ref.Name, Kind: m.profile.Kind, Sleep: m.host.Clock.Sleep, Now: m.host.Clock.Now}
	if m.profile.Kind == coremachine.Mac {
		d.Kept = engineKept(m.engineAPI())
		d.Holder = engineHolder(m.engineAPI())
	}
	if m.powerPort != nil && (m.profile.Caps.Thermal || m.profile.Caps.Battery) {
		d.Power = m.powerPort
	}
	return d
}
