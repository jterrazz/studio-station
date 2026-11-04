package calls

import (
	"context"

	"github.com/jterrazz/studio-station/src/core/call"
	"github.com/jterrazz/studio-station/src/core/confirm"
	"github.com/jterrazz/studio-station/src/core/fail"
	"github.com/jterrazz/studio-station/src/core/section"
	"github.com/jterrazz/studio-station/src/settings"
)

type Shortcuts interface {
	Git(ctx context.Context, name string, args []string) error
	// value: a shortcut that deletes every container, image, or both.
	Containers(ctx context.Context, name string) (settings.ContainersOutcome, error)
}

type GitIn struct {
	Name  string
	Words []string
}

var Git = call.Call[Deps, GitIn, struct{}]{
	Name: "run.git", Section: section.Settings, Where: call.AtTarget, Applies: settings.Applies,
	Run: func(ctx context.Context, d Deps, in GitIn, _ call.Progress) (struct{}, error) {
		return struct{}{}, d.Shortcuts.Git(ctx, in.Name, in.Words)
	},
}

type ContainersIn struct {
	Name    string
	Confirm string
}

var Containers = call.Call[Deps, ContainersIn, settings.ContainersOutcome]{
	Name: "run.containers", Section: section.Settings, Where: call.AtTarget, Applies: settings.Applies, Confirm: confirm.Remove,
	Errors: []fail.Code{confirm.YesNeeded},
	Run: func(ctx context.Context, d Deps, in ContainersIn, _ call.Progress) (settings.ContainersOutcome, error) {
		if err := confirm.Require(confirm.For(confirm.Remove, d.Kind), in.Name, in.Confirm); err != nil {
			return settings.ContainersOutcome{}, err
		}
		return d.Shortcuts.Containers(ctx, in.Name)
	},
}
