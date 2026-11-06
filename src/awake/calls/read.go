package calls

import (
	"context"

	"github.com/jterrazz/studio-station/src/awake"
	"github.com/jterrazz/studio-station/src/core/call"
)

type ReadIn struct{}

var Read = call.Watch[Deps, ReadIn, awake.Report]{
	Name: "awake.read", Where: call.InEngine, Applies: awake.Applies,
	Run: func(ctx context.Context, d Deps, _ ReadIn, _ call.Progress) (<-chan call.Update[awake.Report], error) {
		if err := d.Keeper.Check(); err != nil {
			return nil, err
		}
		servers := d.servers(ctx)
		out := make(chan awake.Report)
		go func() {
			defer close(out)
			for r := range d.Keeper.Read(ctx) {
				r.Mac, r.Caps, r.Servers = d.Mac, d.Caps, servers
				select {
				case out <- r:
				case <-ctx.Done():
					return
				}
			}
		}()
		return call.Updates(ctx, out), nil
	},
}
