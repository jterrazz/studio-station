package calls

import (
	"context"

	"github.com/jterrazz/studio-station/src/core/call"
	"github.com/jterrazz/studio-station/src/core/confirm"
	"github.com/jterrazz/studio-station/src/core/fail"
	"github.com/jterrazz/studio-station/src/core/machine"
	"github.com/jterrazz/studio-station/src/machines"
)

type AddIn struct {
	Name string
	// value: the login and the key ssh uses there; empty takes the owner's.
	User, Key string
}

var Add = call.Call[Deps, AddIn, struct{}]{
	Name: "machines.add", Where: call.InProcess, Applies: machine.Anywhere,
	Run: func(ctx context.Context, d Deps, in AddIn, _ call.Progress) (struct{}, error) {
		if err := d.Machines.CheckHost(); err != nil {
			return struct{}{}, err
		}
		return struct{}{}, d.Machines.AddHost(ctx, in.Name, machines.Host{User: in.User, Key: in.Key})
	},
}

type ForgetIn struct {
	Name    string
	Confirm string
}

var Forget = call.Call[Deps, ForgetIn, struct{}]{
	Name: "machines.forget", Where: call.InProcess, Applies: machine.Anywhere, Confirm: confirm.Forget,
	Errors: []fail.Code{confirm.YesNeeded},
	Run: func(ctx context.Context, d Deps, in ForgetIn, _ call.Progress) (struct{}, error) {
		if err := d.Machines.CheckHost(); err != nil {
			return struct{}{}, err
		}
		if err := d.Machines.CheckRemoteHost(in.Name, "forget"); err != nil {
			return struct{}{}, err
		}
		if err := confirm.Require(confirm.For(confirm.Forget, d.Kind), in.Name, in.Confirm); err != nil {
			return struct{}{}, err
		}
		return struct{}{}, d.Machines.ForgetHost(ctx, in.Name)
	},
}

type UnlockIn struct {
	Name string
	// value: the pre-boot session asks the password at a terminal.
	Interactive bool
}

var Unlock = call.Call[Deps, UnlockIn, struct{}]{
	Name: "machines.unlock", Where: call.InProcess, Applies: machine.Anywhere,
	Run: func(ctx context.Context, d Deps, in UnlockIn, _ call.Progress) (struct{}, error) {
		if err := d.Machines.CheckHost(); err != nil {
			return struct{}{}, err
		}
		if err := d.Machines.CheckRemoteHost(in.Name, "unlock"); err != nil {
			return struct{}{}, err
		}
		return struct{}{}, d.Machines.Unlock(ctx, in.Name, in.Interactive)
	},
}

type SSHIn struct{ Name string }

var SSH = call.Call[Deps, SSHIn, machines.SSHTarget]{
	Name: "machines.ssh", Where: call.InProcess, Applies: machine.Anywhere,
	Run: func(ctx context.Context, d Deps, in SSHIn, _ call.Progress) (machines.SSHTarget, error) {
		return d.Machines.SSH(ctx, in.Name)
	},
}
