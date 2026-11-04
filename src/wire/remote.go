package wire

import (
	"context"
	"encoding/json"
	"io"
	"slices"

	"github.com/spf13/cobra"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/core/call"
	"github.com/jterrazz/studio-station/src/core/confirm"
	"github.com/jterrazz/studio-station/src/core/fail"
	coremachine "github.com/jterrazz/studio-station/src/core/machine"
	"github.com/jterrazz/studio-station/src/domain/machines"
	"github.com/jterrazz/studio-station/src/domain/services"
	machinescalls "github.com/jterrazz/studio-station/src/machines/calls"
	"github.com/jterrazz/studio-station/src/platform/local"
	portscalls "github.com/jterrazz/studio-station/src/ports/calls"
	"github.com/jterrazz/studio-station/src/remote"
	"github.com/jterrazz/studio-station/src/remote/adapter/sshstdio"
	"github.com/jterrazz/studio-station/src/remote/adapter/unix"
	"github.com/jterrazz/studio-station/src/remote/contract"
	"github.com/jterrazz/studio-station/src/remote/each"
	"github.com/jterrazz/studio-station/src/remote/serve"
	enginecmd "github.com/jterrazz/studio-station/src/surface/cli/engine"
	engineattach "github.com/jterrazz/studio-station/src/surface/cli/engine/attach"
	"github.com/jterrazz/studio-station/src/surface/cli/kit"
)

// value: the Router of this run, set once it is built, for what reaches
// another machine from inside a call (the fleet's fan-out, a release there).
type late struct{ router *remote.Router }

func (m machine) router(env kit.Env) *remote.Router {
	var macs contract.EndpointDialer
	if spawn, ok := m.host.Run.(local.Spawner); ok {
		macs = sshstdio.New(spawn)
	}
	r := remote.NewRouter(m.registry(env), m.profile, remote.Reach{
		Macs:    macs,
		Engine:  unix.New(guestRelay{m.hostDaemon()}, func() contract.Hello { return m.far.router.Served() }),
		Locate:  m.locate,
		Holder:  m.holder,
		Version: Version(),
	})
	m.far.router = r
	return r
}

// invariant: a bare name is a Mac when this Mac holds its host record, a VM
// of this Mac when it holds its VM record; any other name is unknown here.
func (m machine) locate(name string) (coremachine.Ref, bool) {
	found, err := m.machines(false).Lookup(context.Background(), name)
	switch {
	case err != nil:
		return coremachine.Ref{Name: name}, false
	case found.Kind != catalog.Host:
		return coremachine.Ref{Name: name, On: m.profile.Ref.Name}, true
	}
	return coremachine.Ref{Name: name}, true
}

// invariant: a VM of another Mac is found in that Mac's own list, asked of
// every other Mac at once; a name none of them holds is unknown, and while a
// Mac is silent the VM may be its own, so that silence is the answer.
func (m machine) holder(ctx context.Context, vm string) (string, error) {
	r := m.far.router
	others := m.machines(false).OtherHosts()
	got := each.All(ctx, others, func(ctx context.Context, mac string) (bool, error) {
		out, err := call.Read(ctx, r, coremachine.Ref{Name: mac}, machinescalls.List, machinescalls.ListIn{Here: true}, nil)
		if err != nil {
			return false, err
		}
		for _, host := range out.Fleet.Hosts {
			if slices.ContainsFunc(host.VMs, func(v machines.VM) bool { return v.Name == vm }) {
				return true, nil
			}
		}
		return false, nil
	})
	for _, mac := range others {
		if got[mac].Err == nil && got[mac].Value {
			return mac, nil
		}
	}
	for _, mac := range others {
		if err := got[mac].Err; err != nil {
			return "", err
		}
	}
	return "", &machines.UnknownMachineError{Name: vm}
}

type guestRelay struct{ d hostDaemon }

func (g guestRelay) GuestExec(ctx context.Context, vm, line, stdin string) (stdout string, code int, err error) {
	out, err := g.d.GuestExec(ctx, vm, line, stdin)
	return out.Stdout, out.ExitCode, err
}

func (m machine) askFleet(ctx context.Context, macs []string, answered func(string, machines.HostView)) {
	r := m.far.router
	for got := range each.Do(ctx, macs, func(ctx context.Context, mac string) (machines.HostView, error) {
		hello, err := r.Hello(ctx, mac)
		if err != nil {
			return machines.HostView{}, err
		}
		out, err := call.Read(ctx, r, coremachine.Ref{Name: mac}, machinescalls.List, machinescalls.ListIn{Here: true}, nil)
		if err != nil || len(out.Fleet.Hosts) == 0 {
			return machines.HostView{}, orNone(err)
		}
		view := out.Fleet.Hosts[0]
		view.StationVersion = hello.Version
		return view, nil
	}) {
		if got.Err == nil {
			answered(got.Mac, got.Value)
		}
	}
}

func (m machine) askServices(ctx context.Context, hosts []string) map[string]services.HostView {
	r := m.far.router
	answers := map[string]services.HostView{}
	for host, got := range each.All(ctx, hosts, func(ctx context.Context, host string) (services.HostView, error) {
		out, err := call.Invoke(ctx, r, coremachine.Ref{Name: host}, portscalls.List, portscalls.ListIn{Here: true}, nil)
		if err != nil || len(out.Hosts) == 0 {
			return services.HostView{}, orNone(err)
		}
		return out.Hosts[0], nil
	}) {
		if got.Err == nil {
			answers[host] = got.Value
		}
	}
	return answers
}

func (m machine) releaseOn(ctx context.Context, host, target string, yes bool) (services.Lease, error) {
	in := portscalls.ReleaseIn{Target: target}
	if yes {
		in.Confirm = confirm.Yes
	}
	out, err := call.Invoke(ctx, m.far.router, coremachine.Ref{Name: host}, portscalls.Release, in, nil)
	return out.Lease, err
}

func orNone(err error) error {
	if err != nil {
		return err
	}
	return fail.New(fail.Unreachable, call.Unreachable, "", nil)
}

// invariant: a server runs each frame through this run's Router: a call
// for this machine here, one for its VM over its engine's connection.
type served struct{ r *remote.Router }

func (s served) Hello() contract.Hello { return s.r.Served() }

func (s served) Invoke(ctx context.Context, target coremachine.Ref, name string, in json.RawMessage, p call.Progress) (any, error) {
	return s.r.Serve(ctx, target, name, in, p)
}

func (s served) Watch(ctx context.Context, target coremachine.Ref, name string, in json.RawMessage, p call.Progress) (<-chan call.Update[any], error) {
	return s.r.ServeWatch(ctx, target, name, in, p)
}

func serveFrames(env kit.Env) func(ctx context.Context, in io.Reader, out io.Writer) error {
	return func(ctx context.Context, in io.Reader, out io.Writer) error {
		r, _ := env.Calls.(*remote.Router)
		return serve.Stream(ctx, in, out, served{r}, func(err error) *fail.Error { return kit.Translate(err, "") })
	}
}

func engineCommand(env kit.Env) *cobra.Command {
	return enginecmd.Command(env, engineattach.Command(engineattach.Deps{Serve: serveFrames(env)}, env))
}

// value: a run that serves frames on its stdout: nothing else of it may write there.
func serving(args []string) bool {
	return len(args) >= 2 && (args[0] == "engine" && args[1] == "attach" || args[0] == "guest" && args[1] == "serve")
}
