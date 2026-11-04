package calls

import (
	"context"

	"github.com/jterrazz/studio-station/src/core/call"
	"github.com/jterrazz/studio-station/src/core/machine"
	"github.com/jterrazz/studio-station/src/machines"
)

type Deps struct {
	Machines Machines
	Kind     machine.Kind
}

type Machines interface {
	Fleet(ctx context.Context, hereOnly bool) (machines.Fleet, error)
	// value: rewrites the owner's ssh blocks for the fleet just read.
	SSHBlocks(ctx context.Context, f machines.Fleet) error
	CheckCreate(ctx context.Context, r machines.CreateRequest) (machines.CreateRequest, error)
	Create(ctx context.Context, r machines.CreateRequest, progress func(machines.Step)) (machines.Created, error)
	CheckStart(name string) error
	Start(ctx context.Context, name string) error
	CheckStop(name string) error
	Stop(ctx context.Context, name string) error
	// value: refuses a name that is no VM of this host, worded for verb.
	CheckVM(name, verb string) error
	Delete(ctx context.Context, name string) (machines.DeleteReport, error)
	// value: whether name is a VM, refusing a temporary one.
	CheckRestart(name string) (vm bool, err error)
	Restart(ctx context.Context, name string) (machines.RestartReport, error)
	CheckHost() error
	// value: refuses a name that is no other registered host, worded for verb.
	CheckRemoteHost(name, verb string) error
	AddHost(ctx context.Context, name string, h machines.Host) error
	ForgetHost(ctx context.Context, name string) error
	// value: refuses off a terminal, where nobody can type the password.
	Unlock(ctx context.Context, name string, interactive bool) error
	// value: refuses a name that is no known machine.
	CheckRemote(name string) error
	Probe(ctx context.Context, name string) (machines.ProbeReport, error)
}

func All(d Deps) []call.Handler {
	return []call.Handler{
		call.Bind(List, d), call.Bind(Create, d), call.Bind(Start, d), call.Bind(Stop, d), call.Bind(Delete, d),
		call.Bind(Restart, d), call.Bind(Add, d), call.Bind(Forget, d), call.Bind(Unlock, d), call.Bind(Probe, d),
	}
}
