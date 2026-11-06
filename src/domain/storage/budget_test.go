package storage

import (
	"context"
	"testing"
	"testing/fstest"
	"time"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/system"
	"github.com/jterrazz/studio-station/src/system/systemtest"
)

type realClock struct{}

func (realClock) Now() time.Time { return time.Unix(0, 0) }

func (realClock) Sleep(ctx context.Context, d time.Duration) error {
	waited, cancel := context.WithTimeout(ctx, d)
	defer cancel()
	<-waited.Done()
	return ctx.Err()
}

type slowFS struct {
	systemtest.Disk
	release chan struct{}
}

func (f slowFS) Sizes(ctx context.Context, roots []string, each func(int, int64)) {
	slow := -1
	for i, p := range roots {
		if p == "/h/slow/tree" {
			slow = i
			continue
		}
		f.FS.Sizes(ctx, []string{p}, func(_ int, b int64) { each(i, b) })
	}
	if slow < 0 {
		return
	}
	select {
	case <-f.release:
		f.FS.Sizes(ctx, roots[slow:slow+1], func(_ int, b int64) { each(slow, b) })
	case <-ctx.Done():
		each(slow, -1)
	}
}

func TestARowPastTheBudgetIsUnknownAndDelaysNoOtherRow(t *testing.T) {
	fs := slowFS{Disk: systemtest.Disk{FS: systemtest.NewFS()}, release: make(chan struct{})}
	defer close(fs.release)
	fs.Files["h/slow/tree/f"] = &fstest.MapFile{Data: []byte("x")}
	fs.Files["h/fast/tree/f"] = &fstest.MapFile{Data: []byte("abc")}
	tools := []catalog.Tool{
		{Name: "slow", Installs: catalog.HostOnly(catalog.Brew("slow")), Storage: catalog.Disk{Paths: []catalog.Stored{{Path: "~/slow/*", Level: catalog.LevelCache}}}},
		{Name: "fast", Installs: catalog.HostOnly(catalog.Brew("fast")), Storage: catalog.Disk{Paths: []catalog.Stored{{Path: "~/fast/*", Level: catalog.LevelCache}}}},
	}
	s := New(system.Host{FS: fs.FS, Clock: realClock{}}, tools, Options{Disk: fs, Here: catalog.Host, Expand: Expander{Home: "/h"}, Budget: 30 * time.Millisecond})

	waited, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	listed := make(chan Report, 1)
	go func() {
		r, _ := s.List(context.Background())
		listed <- r
	}()
	var report Report
	select {
	case report = <-listed:
	case <-waited.Done():
		t.Fatal("List waited on a stuck tree past its budget")
	}
	got := map[string]Row{}
	for _, r := range report.Rows {
		got[r.Entry.Name] = r
	}
	if slow := got["slow"]; !slow.Unknown || slow.Bytes[catalog.LevelCache] != Unknown || slow.Within != 30*time.Millisecond {
		t.Errorf("the stuck row answered %+v, want unknown sizes, not sized within the walk's budget", slow)
	}
	if fast := got["fast"]; fast.Unknown || fast.Bytes[catalog.LevelCache] != 3 {
		t.Errorf("the fast row answered %+v, want its 3 bytes", fast)
	}
}
