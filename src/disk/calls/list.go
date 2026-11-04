package calls

import (
	"context"

	"github.com/jterrazz/studio-station/src/core/call"
	"github.com/jterrazz/studio-station/src/core/section"
	"github.com/jterrazz/studio-station/src/disk"
)

type ListIn struct{}

var List = call.Watch[Deps, ListIn, disk.Report]{
	Name: "storage.list", Section: section.Disk, Where: call.AtTarget, Applies: disk.Applies,
	Run: func(ctx context.Context, d Deps, _ ListIn, _ call.Progress) (<-chan call.Update[disk.Report], error) {
		return call.Updates(ctx, d.Storage.Watch(ctx)), nil
	},
}
