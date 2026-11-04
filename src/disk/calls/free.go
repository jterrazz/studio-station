package calls

import (
	"context"

	"github.com/jterrazz/studio-station/src/core/call"
	"github.com/jterrazz/studio-station/src/core/fail"
	"github.com/jterrazz/studio-station/src/core/section"
	"github.com/jterrazz/studio-station/src/disk"
)

type FreeIn struct {
	// value: no app frees every Can rebuild row here.
	Apps []string
}

var Free = call.Call[Deps, FreeIn, []disk.Outcome]{
	Name: "disk.free", Section: section.Disk, Where: call.AtTarget, Applies: disk.Applies,
	Errors: []fail.Code{Unknown, Elsewhere, NothingToFree},
	Run: func(ctx context.Context, d Deps, in FreeIn, _ call.Progress) ([]disk.Outcome, error) {
		out, err := d.Storage.Clean(ctx, in.Apps)
		return out, coded(err)
	},
}
