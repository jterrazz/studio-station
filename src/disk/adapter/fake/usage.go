package fake

import (
	"context"
	"strings"
	"sync"
)

type Usage struct {
	mu          sync.Mutex
	files       map[string]int64
	total, free int64
	running     map[string]bool
}

func NewUsage(files map[string]int64, total, free int64, running ...string) *Usage {
	// value: a disk holding files by their absolute path and length, on a
	// volume of total bytes with free left.
	u := &Usage{files: map[string]int64{}, total: total, free: free, running: map[string]bool{}}
	for p, n := range files {
		u.files[p] = n
	}
	for _, r := range running {
		u.running[r] = true
	}
	return u
}

func (u *Usage) Sizes(ctx context.Context, roots []string, each func(int, int64)) {
	u.mu.Lock()
	defer u.mu.Unlock()
	for i, root := range roots {
		if ctx.Err() != nil {
			each(i, -1)
			continue
		}
		var total int64
		for p, n := range u.files {
			if p == root || strings.HasPrefix(p, root+"/") {
				total += n
			}
		}
		each(i, total)
	}
}

func (u *Usage) Volume(context.Context) (total, free int64, err error) {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.total, u.free, nil
}

func (u *Usage) Erase(_ context.Context, paths []string) error {
	u.mu.Lock()
	defer u.mu.Unlock()
	for _, root := range paths {
		for p, n := range u.files {
			if p == root || strings.HasPrefix(p, root+"/") {
				delete(u.files, p)
				u.free += n
			}
		}
	}
	return nil
}

func (u *Usage) Running(_ context.Context, process string) bool {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.running[process]
}
