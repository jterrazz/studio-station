package calls

import (
	"context"

	"github.com/jterrazz/studio-station/src/core/call"
	"github.com/jterrazz/studio-station/src/core/confirm"
	"github.com/jterrazz/studio-station/src/core/fail"
	"github.com/jterrazz/studio-station/src/core/machine"
	"github.com/jterrazz/studio-station/src/machines"
)

type VMIn struct{ VM string }

var Start = call.Call[Deps, VMIn, struct{}]{
	Name: "machines.start", Where: call.InEngine, Applies: machine.Anywhere,
	Run: func(ctx context.Context, d Deps, in VMIn, _ call.Progress) (struct{}, error) {
		if err := d.Machines.CheckStart(in.VM); err != nil {
			return struct{}{}, err
		}
		return struct{}{}, d.Machines.Start(ctx, in.VM)
	},
}

var Stop = call.Call[Deps, VMIn, struct{}]{
	// invariant: stop never destroys, so it asks nothing yet; a temporary
	// VM is refused, since its stop would be its end.
	Name: "machines.stop", Where: call.InEngine, Applies: machine.Anywhere,
	Run: func(ctx context.Context, d Deps, in VMIn, _ call.Progress) (struct{}, error) {
		if err := d.Machines.CheckStop(in.VM); err != nil {
			return struct{}{}, err
		}
		return struct{}{}, d.Machines.Stop(ctx, in.VM)
	},
}

type DeleteIn struct {
	VM      string
	Confirm string
}

var Delete = call.Call[Deps, DeleteIn, machines.DeleteReport]{
	Name: "machines.delete", Where: call.InEngine, Applies: machine.Anywhere, Confirm: confirm.Delete,
	Errors: []fail.Code{confirm.Needed, confirm.Mismatch},
	Run: func(ctx context.Context, d Deps, in DeleteIn, _ call.Progress) (machines.DeleteReport, error) {
		if err := d.Machines.CheckVM(in.VM, "delete"); err != nil {
			return machines.DeleteReport{}, err
		}
		if err := confirm.Require(confirm.For(confirm.Delete, machine.Workspace), in.VM, in.Confirm); err != nil {
			return machines.DeleteReport{}, err
		}
		return d.Machines.Delete(ctx, in.VM)
	},
}

type RestartIn struct {
	// value: a host (a FileVault-aware reboot) or a lasting VM (a poweroff
	// and a start).
	Name    string
	Confirm string
}

var Restart = call.Call[Deps, RestartIn, machines.RestartReport]{
	Name: "machines.restart", Where: call.AtTarget, Applies: machine.Anywhere, Confirm: confirm.RestartMac,
	Errors: []fail.Code{confirm.YesNeeded},
	Run: func(ctx context.Context, d Deps, in RestartIn, _ call.Progress) (machines.RestartReport, error) {
		if err := d.Machines.CheckHost(); err != nil {
			return machines.RestartReport{}, err
		}
		if _, err := d.Machines.CheckRestart(in.Name); err != nil {
			return machines.RestartReport{}, err
		}
		if err := confirm.Require(confirm.For(confirm.RestartMac, d.Kind), in.Name, in.Confirm); err != nil {
			return machines.RestartReport{}, err
		}
		return d.Machines.Restart(ctx, in.Name)
	},
}
