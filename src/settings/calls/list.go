package calls

import (
	"context"
	"slices"

	"github.com/jterrazz/studio-station/src/core/call"
	"github.com/jterrazz/studio-station/src/core/section"
	"github.com/jterrazz/studio-station/src/settings"
)

type ListIn struct{}

var List = call.Watch[Deps, ListIn, []settings.Row]{
	Name: "settings.list", Section: section.Settings, Where: call.AtTarget, Applies: settings.Applies,
	Run: func(ctx context.Context, d Deps, _ ListIn, _ call.Progress) (<-chan call.Update[[]settings.Row], error) {
		// invariant: the first value is every setting, those that live here
		// being read; each fills as its Applier answers, so the list costs its
		// slowest read.
		items := d.Settings.Items()
		off := offHere(d)
		rows := make([]settings.Row, len(items))
		answers := make(chan struct {
			i   int
			row settings.Row
		}, len(items))
		left := 0
		for i, it := range items {
			if !d.Settings.Lives(it) || off[it.Name] {
				rows[i], _ = judge(ctx, d, it, off)
				continue
			}
			rows[i] = settings.Row{Name: it.Name, Group: settings.GroupOf(it), State: settings.StateReading}
			left++
			go func() {
				row, _ := judge(ctx, d, it, off)
				answers <- struct {
					i   int
					row settings.Row
				}{i, row}
			}()
		}
		out := make(chan []settings.Row)
		go func() {
			defer close(out)
			send := func() bool {
				select {
				case out <- slices.Clone(rows):
					return true
				case <-ctx.Done():
					return false
				}
			}
			if !send() {
				return
			}
			for ; left > 0; left-- {
				a := <-answers
				rows[a.i] = a.row
				if !send() {
					return
				}
			}
		}()
		return call.Updates(ctx, out), nil
	},
}
