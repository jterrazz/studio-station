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

type StartOut struct {
	// value: the workspace already ran; nothing was started.
	Already bool
}

var Start = call.Call[Deps, VMIn, StartOut]{
	Name: "machines.start", Where: call.InEngine, Applies: machine.Anywhere,
	Run: func(ctx context.Context, d Deps, in VMIn, _ call.Progress) (StartOut, error) {
		if err := d.Machines.CheckStart(in.VM); err != nil {
			return StartOut{}, err
		}
		if err := d.Machines.EngineUp(ctx); err != nil {
			return StartOut{}, err
		}
		if running, err := d.Machines.Running(ctx, in.VM); err != nil || running {
			return StartOut{Already: running}, err
		}
		return StartOut{}, d.Machines.Start(ctx, in.VM)
	},
}

type StopIn struct {
	VM      string
	Confirm string
}

var Stop = call.Call[Deps, StopIn, struct{}]{
	// invariant: a stop keeps the workspace, throwaway or permanent (SP-5),
	// and asks yes or no first; one already stopped asks nothing.
	Name: "machines.stop", Where: call.InEngine, Applies: machine.Anywhere, Confirm: confirm.Stop,
	Errors: []fail.Code{confirm.YesNeeded},
	Run: func(ctx context.Context, d Deps, in StopIn, _ call.Progress) (struct{}, error) {
		if err := d.Machines.CheckStop(in.VM); err != nil {
			return struct{}{}, err
		}
		if err := d.Machines.EngineUp(ctx); err != nil {
			return struct{}{}, err
		}
		if running, err := d.Machines.Running(ctx, in.VM); err != nil || !running {
			return struct{}{}, err
		}
		if err := confirm.Require(confirm.For(confirm.Stop, machine.Workspace), in.VM, in.Confirm); err != nil {
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
		if err := d.Machines.EngineUp(ctx); err != nil {
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
		vm, err := d.Machines.CheckRestart(in.Name)
		if err != nil {
			return machines.RestartReport{}, err
		}
		gesture := confirm.RestartMac
		if vm {
			gesture = confirm.Restart
		}
		if err := confirm.Require(confirm.For(gesture, d.Kind), in.Name, in.Confirm); err != nil {
			return machines.RestartReport{}, err
		}
		return d.Machines.Restart(ctx, in.Name)
	},
}
