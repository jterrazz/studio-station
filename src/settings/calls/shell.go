package calls

import (
	"context"

	"github.com/jterrazz/studio-station/src/core/call"
	"github.com/jterrazz/studio-station/src/core/machine"
)

type ZshIn struct {
	// value: the completion script cobra wrote, which the block evaluates too.
	Completion string
}

var Zsh = call.Call[Deps, ZshIn, string]{
	Name: "shell.zsh", Where: call.AtTarget, Applies: machine.Anywhere,
	Run: func(ctx context.Context, d Deps, in ZshIn, _ call.Progress) (string, error) {
		return d.Shell.Zsh(ctx, in.Completion)
	},
}
