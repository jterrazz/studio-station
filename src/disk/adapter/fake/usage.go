package fake

import (
	"context"
	"maps"
	"slices"
	"strings"

	"github.com/jterrazz/studio-station/src/world"
)

type Usage struct {
	// invariant: the volume of one Mac of the world and the files on it.
	w   *world.World
	mac string
}

func UsageOf(w *world.World, mac string) *Usage { return &Usage{w: w, mac: mac} }

func NewUsage(files map[string]int64, total, free int64, running ...string) *Usage {
	// value: a disk holding files by their absolute path and length, on a
	// volume of total bytes with free left.
	w := &world.World{Here: "here"}
	w.Mac("here").Disk = &world.Disk{Total: total, Free: free, Files: maps.Clone(files), Running: running}
	return UsageOf(w, "here")
}

func (u *Usage) disk() *world.Disk {
	m := u.w.Mac(u.mac)
	if m.Disk == nil {
		m.Disk = &world.Disk{}
	}
	if m.Disk.Files == nil {
		m.Disk.Files = map[string]int64{}
	}
	return m.Disk
}

func under(p, root string) bool { return p == root || strings.HasPrefix(p, root+"/") }

func (u *Usage) Sizes(ctx context.Context, roots []string, each func(int, int64)) {
	// invariant: each root is told with the world unlocked, since its caller
	// may wait on a reader that needs the world.
	totals := make([]int64, len(roots))
	u.w.Lock()
	for i, root := range roots {
		for p, n := range u.disk().Files {
			if under(p, root) {
				totals[i] += n
			}
		}
	}
	u.w.Unlock()
	for i := range roots {
		if ctx.Err() != nil {
			each(i, -1)
			continue
		}
		each(i, totals[i])
	}
}

func (u *Usage) Volume(context.Context) (total, free int64, err error) {
	u.w.Lock()
	defer u.w.Unlock()
	d := u.disk()
	return d.Total, d.Free, nil
}

func (u *Usage) Erase(_ context.Context, paths []string) error {
	u.w.Lock()
	defer u.w.Unlock()
	d := u.disk()
	for _, root := range paths {
		for p, n := range d.Files {
			if under(p, root) {
				delete(d.Files, p)
				d.Free += n
			}
		}
	}
	return nil
}

func (u *Usage) Running(_ context.Context, process string) bool {
	u.w.Lock()
	defer u.w.Unlock()
	return slices.Contains(u.disk().Running, process)
}
