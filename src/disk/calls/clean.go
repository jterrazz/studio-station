package calls

import (
	"context"

	"github.com/jterrazz/studio-station/src/core/call"
	"github.com/jterrazz/studio-station/src/core/section"
	"github.com/jterrazz/studio-station/src/disk"
)

type CleanIn struct {
	// value: no name cleans every tool here.
	Names []string
}

var Clean = call.Call[Deps, CleanIn, []disk.Outcome]{
	Name: "storage.clean", Section: section.Disk, Where: call.AtTarget, Applies: disk.Applies,
	Run: func(ctx context.Context, d Deps, in CleanIn, _ call.Progress) ([]disk.Outcome, error) {
		return d.Storage.Clean(ctx, in.Names)
	},
}
