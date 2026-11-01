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

type slowFS struct {
	*systemtest.FS
	release chan struct{}
}

func (f slowFS) DirSize(p string) int64 {
	if p == "/h/slow/tree" {
		<-f.release
	}
	return f.FS.DirSize(p)
}

func TestARowPastTheBudgetIsUnknownAndDelaysNoOtherRow(t *testing.T) {
	fs := slowFS{FS: systemtest.NewFS(), release: make(chan struct{})}
	defer close(fs.release)
	fs.Files["h/slow/tree/f"] = &fstest.MapFile{Data: []byte("x")}
	fs.Files["h/fast/tree/f"] = &fstest.MapFile{Data: []byte("abc")}
	tools := []catalog.Tool{
		{Name: "slow", Installs: catalog.HostOnly(catalog.Brew("slow")), Storage: catalog.Storage{Paths: []catalog.Stored{{Path: "~/slow/*", Level: catalog.LevelCache}}}},
		{Name: "fast", Installs: catalog.HostOnly(catalog.Brew("fast")), Storage: catalog.Storage{Paths: []catalog.Stored{{Path: "~/fast/*", Level: catalog.LevelCache}}}},
	}
	s := New(system.Host{FS: fs}, tools, Options{Here: catalog.Host, Expand: Expander{Home: "/h"}, Budget: 30 * time.Millisecond})

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
	if slow := got["slow"]; !slow.Unknown || slow.Bytes[catalog.LevelCache] != Unknown {
		t.Errorf("the stuck row answered %+v, want unknown sizes", slow)
	}
	if fast := got["fast"]; fast.Unknown || fast.Bytes[catalog.LevelCache] != 3 {
		t.Errorf("the fast row answered %+v, want its 3 bytes", fast)
	}
}
