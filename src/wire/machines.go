package wire

import (
	"context"
	"errors"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/jterrazz/studio-station/src/core/fail"
	"github.com/jterrazz/studio-station/src/domain/images"
	"github.com/jterrazz/studio-station/src/domain/machines"
	"github.com/jterrazz/studio-station/src/domain/remote"
	"github.com/jterrazz/studio-station/src/machines/adapter/tart"
	"github.com/jterrazz/studio-station/src/surface/cli/kit"
	machinescmd "github.com/jterrazz/studio-station/src/surface/cli/machines"
	machinesadd "github.com/jterrazz/studio-station/src/surface/cli/machines/add"
	machinescreate "github.com/jterrazz/studio-station/src/surface/cli/machines/create"
	machinesdelete "github.com/jterrazz/studio-station/src/surface/cli/machines/delete"
	machinesforget "github.com/jterrazz/studio-station/src/surface/cli/machines/forget"
	machinespipe "github.com/jterrazz/studio-station/src/surface/cli/machines/pipe"
	machinesprobe "github.com/jterrazz/studio-station/src/surface/cli/machines/probe"
	machinesrestart "github.com/jterrazz/studio-station/src/surface/cli/machines/restart"
	machinesstart "github.com/jterrazz/studio-station/src/surface/cli/machines/start"
	machinesstop "github.com/jterrazz/studio-station/src/surface/cli/machines/stop"
	machinessupervise "github.com/jterrazz/studio-station/src/surface/cli/machines/supervise"
	machinesunlock "github.com/jterrazz/studio-station/src/surface/cli/machines/unlock"
)

const (
	defaultOpenClawPort = 18789
	defaultHermesPort   = 8642
)

func (m machine) machines(_ bool) *machines.Service {
	return machines.New(m.host, tart.New(m.host.Run), m.store, hostNodes{tailnet: m.tailnet()}, fleetImages{service: m.images()}, m.hostDaemon(), leaseReleaser{m: m}, machines.Options{
		Ports: machines.Ports{
			OpenClaw: m.port(defaultOpenClawPort, "OPENCLAW_GATEWAY_PORT"),
			Hermes:   m.port(defaultHermesPort, "HERMES_GATEWAY_PORT", "API_SERVER_PORT"),
		},
		Here:      m.kind,
		Paths:     m.paths,
		User:      m.host.Env.User(),
		Version:   Version(),
		Remote:    fanOutArgv,
		Hop:       func(host string, words []string) []string { return remote.Argv(host, false, words) },
		GuestPort: m.guestPort(),
	})
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
	return machinescmd.Command(machinescmd.Deps{Machines: service, Here: m.where}, env,
		machinesadd.Command(machinesadd.Deps{Machines: service, Here: m.where}, env),
		machinesforget.Command(machinesforget.Deps{Machines: service, Here: m.where}, env),
		machinescreate.Command(machinescreate.Deps{
			Machines: service, Here: m.where, Ask: createAsker(env),
			Build: m.buildImage, On: createOn(env),
		}, env),
		machinesstart.Command(machinesstart.Deps{Machines: service, Here: m.where}, env),
		machinesstop.Command(machinesstop.Deps{Machines: service, Here: m.where}, env),
		machinesrestart.Command(machinesrestart.Deps{Machines: service, Here: m.where}, env),
		machinesdelete.Command(machinesdelete.Deps{Machines: service, Here: m.where}, env),
		machinesprobe.Command(machinesprobe.Deps{Machines: service, Here: m.where}, env),
		machinesunlock.Command(machinesunlock.Deps{Machines: service}, env),
		machinessupervise.Command(env),
		machinespipe.Command(machinespipe.Deps{Machines: service}, env),
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

func createAsker(env kit.Env) kit.Asker {
	if !env.Interactive {
		return nil
	}
	return env.Prompt
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
			return fail.Silent(fail.Exit(code))
		}
		return nil
	}
}
