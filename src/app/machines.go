package app

import (
	"context"
	"errors"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/jterrazz/studio-station/src/cli/kit"
	machinescmd "github.com/jterrazz/studio-station/src/cli/machines"
	machinesadd "github.com/jterrazz/studio-station/src/cli/machines/add"
	machinescreate "github.com/jterrazz/studio-station/src/cli/machines/create"
	machinesdelete "github.com/jterrazz/studio-station/src/cli/machines/delete"
	machinesforget "github.com/jterrazz/studio-station/src/cli/machines/forget"
	machinesprobe "github.com/jterrazz/studio-station/src/cli/machines/probe"
	machinesrestart "github.com/jterrazz/studio-station/src/cli/machines/restart"
	machinesstart "github.com/jterrazz/studio-station/src/cli/machines/start"
	machinesstop "github.com/jterrazz/studio-station/src/cli/machines/stop"
	machinessupervise "github.com/jterrazz/studio-station/src/cli/machines/supervise"
	machinesunlock "github.com/jterrazz/studio-station/src/cli/machines/unlock"
	"github.com/jterrazz/studio-station/src/domain/images"
	"github.com/jterrazz/studio-station/src/domain/machines"
	"github.com/jterrazz/studio-station/src/domain/remote"
	"github.com/jterrazz/studio-station/src/fail"
	"github.com/jterrazz/studio-station/src/prompt"
	"github.com/jterrazz/studio-station/src/system/tart"
)

const (
	defaultOpenClawPort = 18789
	defaultHermesPort   = 8642
)

func (m machine) machines(interactive bool) *machines.Service {
	return machines.New(m.host, tart.New(m.host.Run), m.store, fleetTailnet{service: m.tailnet(interactive)}, fleetImages{service: m.images()}, machines.Options{
		Ports: machines.Ports{
			OpenClaw: m.port(defaultOpenClawPort, "OPENCLAW_GATEWAY_PORT"),
			Hermes:   m.port(defaultHermesPort, "HERMES_GATEWAY_PORT", "API_SERVER_PORT"),
		},
		Here:     m.kind,
		Paths:    m.paths,
		User:     m.host.Env.User(),
		Version:  Version(),
		Remote:   fanOutArgv,
		Agent:    m.vars["STATION_VM_AGENT"],
		AgentEnv: m.agentEnv(),
	})
}

// invariant: a launchd agent starts with launchd's own PATH and no XDG, so the
// agent of a VM gets the creating process's resolution of both.
func (m machine) agentEnv() map[string]string {
	env := map[string]string{"HOME": m.host.Env.Home()}
	for _, name := range []string{"PATH", "XDG_CONFIG_HOME", "XDG_STATE_HOME", "XDG_CACHE_HOME"} {
		if v := m.vars[name]; v != "" {
			env[name] = v
		}
	}
	return env
}

func (m machine) port(fallback int, names ...string) int {
	for _, name := range names {
		if n, err := strconv.Atoi(m.vars[name]); err == nil && n > 0 {
			return n
		}
	}
	return fallback
}

// hazard: the fleet asks every host at once and cannot answer a password prompt,
// so the ssh call is BatchMode; the words are fixed and need no quoting.
func fanOutArgv(target string, words []string) []string {
	argv := make([]string, 0, 11+len(words))
	argv = append(argv,
		"ssh", "-o", "BatchMode=yes", "-o", "ConnectTimeout=5", target, "--",
		"sh", "-c", `'PATH="$HOME/.local/bin:/usr/local/bin:$PATH" exec station "$@"'`, "station",
	)
	return append(argv, words...)
}

func machinesCommand(env kit.Env, m machine) *cobra.Command {
	service := m.machines(env.Interactive)
	return machinescmd.Command(machinescmd.Deps{Machines: service, Context: m.context}, env,
		machinesadd.Command(machinesadd.Deps{Machines: service, Context: m.context}, env),
		machinesforget.Command(machinesforget.Deps{Machines: service, Context: m.context}, env),
		machinescreate.Command(machinescreate.Deps{
			Machines: service, Context: m.context, Ask: createAsker(env),
			Build: m.buildImage, On: createOn(env),
		}, env),
		machinesstart.Command(machinesstart.Deps{Machines: service, Context: m.context}, env),
		machinesstop.Command(machinesstop.Deps{Machines: service, Context: m.context}, env),
		machinesrestart.Command(machinesrestart.Deps{Machines: service, Context: m.context}, env),
		machinesdelete.Command(machinesdelete.Deps{Machines: service, Context: m.context}, env),
		machinesprobe.Command(machinesprobe.Deps{Machines: service, Context: m.context}, env),
		machinesunlock.Command(machinesunlock.Deps{Machines: service, Context: m.context}, env),
		machinessupervise.Command(machinessupervise.Deps{Machines: service}, env),
	)
}

type fleetImages struct{ service *images.Service }

func (i fleetImages) KeyPath() string { return i.service.KeyPath() }

func (i fleetImages) Latest(ctx context.Context, image string) (tartName, version string, err error) {
	r, err := i.service.Latest(ctx, image)
	var unbuilt *images.NotBuiltError
	if errors.As(err, &unbuilt) {
		return "", "", &machines.NoImageError{Image: image}
	}
	if err != nil {
		return "", "", err
	}
	return images.TartName(r.Name, r.Version), r.Version, nil
}

func createAsker(env kit.Env) prompt.CreateAsker {
	if !env.Interactive {
		return nil
	}
	ask, _ := env.Prompt.(prompt.CreateAsker)
	return ask
}

func (m machine) buildImage(ctx context.Context, image string) error {
	_, err := m.images().Build(ctx, images.BuildRequest{Images: []string{image}}, nil)
	return err
}

// invariant: a host chosen on a terminal runs the very request there, as --on
// would, with the lifetime the owner just answered.
func createOn(env kit.Env) func(ctx context.Context, host, lifetime string) error {
	return func(ctx context.Context, host, lifetime string) error {
		words := append(remote.Words(env.On.Args), "--lifetime", lifetime)
		code, err := env.On.Remote.Run(ctx, host, true, words)
		if err != nil {
			return err
		}
		if code != 0 {
			return fail.Silent(fail.Code(code))
		}
		return nil
	}
}
