package calls

import (
	"context"

	"github.com/jterrazz/studio-station/src/apps"
	"github.com/jterrazz/studio-station/src/core/call"
	"github.com/jterrazz/studio-station/src/core/section"
	"github.com/jterrazz/studio-station/src/disk"
	"github.com/jterrazz/studio-station/src/overview"
	"github.com/jterrazz/studio-station/src/settings"
)

type (
	Snapshot = overview.Snapshot[apps.Report, settings.Report, disk.StrayReport]
	Sections = Deps[apps.Report, settings.Report, disk.StrayReport]
	Overview = Reader[apps.Report, settings.Report, disk.StrayReport]
)

func All(d Sections) []call.Handler {
	return []call.Handler{call.BindWatch(Read, New(d))}
}

type ReadIn struct {
	// value: every shared reading of the run dropped first, so the whole
	// machine is read again.
	Fresh bool
}

var Read = call.Watch[*Overview, ReadIn, Snapshot]{
	Name: "overview.read", Section: section.Overview, Where: call.AtTarget, Applies: overview.Applies,
	Run: func(ctx context.Context, r *Overview, in ReadIn, _ call.Progress) (<-chan call.Update[Snapshot], error) {
		if in.Fresh {
			r.Forget()
		}
		return call.Updates(ctx, r.Watch(ctx)), nil
	},
}
