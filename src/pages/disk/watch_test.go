package disk

import (
	"context"
	"maps"
	"slices"
	"strconv"
	"testing"
	"testing/fstest"
	"time"

	"github.com/jterrazz/studio-station/src/catalog"
	probe "github.com/jterrazz/studio-station/src/core/call/lanes"
	"github.com/jterrazz/studio-station/src/disk"
	"github.com/jterrazz/studio-station/src/domain/storage"
	"github.com/jterrazz/studio-station/src/page"
	"github.com/jterrazz/studio-station/src/system/systemtest"
)

type clock struct{}

func (clock) Now() time.Time { return time.Unix(0, 0) }

func (clock) Sleep(ctx context.Context, d time.Duration) error {
	waited, cancel := context.WithTimeout(ctx, d)
	defer cancel()
	<-waited.Done()
	return ctx.Err()
}

// value: a disk that answers its roots one at a time, 10 ms apart.
type staggered struct{ systemtest.Disk }

func (f staggered) Sizes(ctx context.Context, roots []string, each func(int, int64)) {
	for i, root := range roots {
		_ = clock{}.Sleep(ctx, 10*time.Millisecond)
		f.FS.Sizes(ctx, []string{root}, func(_ int, b int64) { each(i, b) })
	}
}

// value: an inventory that names its tools one at a time, the odd ones not
// installed, so some rows (and the sections they alone fill) leave at the end.
type inventory struct{ names []string }

func (v inventory) Installed(ctx context.Context, each func(map[string]bool)) {
	got := map[string]bool{}
	for i, n := range v.names {
		_ = clock{}.Sleep(ctx, 10*time.Millisecond)
		got[n] = i%2 == 0
		each(maps.Clone(got))
	}
}

func shape(p page.Page) (ids []string, rows map[string]int) {
	rows = map[string]int{}
	for _, s := range p.Sections {
		ids = append(ids, s.ID)
		rows[s.ID] = len(s.Items)
	}
	return ids, rows
}

func TestNoSectionFlickers(t *testing.T) {
	fs := staggered{Disk: systemtest.Disk{FS: systemtest.NewFS()}}
	categories := []catalog.Category{catalog.CategoryPackageManagers, catalog.CategoryRuntimes, catalog.CategoryAIApps, catalog.CategoryAgentSkills}
	tools := make([]catalog.Tool, 0, 40)
	names := make([]string, 0, 40)
	for i := range 40 {
		name := "t" + strconv.Itoa(i)
		fs.Files["h/"+name+"/cache/f"] = &fstest.MapFile{Data: []byte("abc")}
		tools = append(tools, catalog.Tool{Name: name, Category: categories[i%len(categories)], Installs: catalog.HostOnly(catalog.Shell(name)),
			Storage: catalog.Disk{Paths: []catalog.Stored{{Path: "~/" + name + "/*", Level: catalog.LevelCache}}}})
		names = append(names, name)
	}
	pool := probe.NewPool(context.Background(), clock{}, 4, 1)
	defer pool.Close()
	host := systemtest.Host(systemtest.NewRunner(t), fs.FS, systemtest.Env{}, systemtest.NewNet(), &systemtest.Clock{})
	host.FS, host.Clock = fs, clock{}
	s := storage.New(host, tools, storage.Options{Disk: fs, Pool: pool, Here: catalog.Host, Expand: storage.Expander{Home: "/h"}, Budget: 5 * time.Second, Inventory: inventory{names: names}})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var pages []page.Page
	for r := range s.Watch(ctx) {
		pages = append(pages, Read(disk.Reading{Rows: disk.Classify(r)}, page.Context{Kind: "host"}, false))
	}
	if len(pages) < 5 {
		t.Fatalf("the watch sent %d values, want one per staggered answer", len(pages))
	}
	for i := 1; i < len(pages)-1; i++ {
		before, beforeRows := shape(pages[i-1])
		after, afterRows := shape(pages[i])
		for _, id := range before {
			if !slices.Contains(after, id) {
				t.Fatalf("value %d: section %s left before the final value", i, id)
			}
			if afterRows[id] < beforeRows[id] {
				t.Fatalf("value %d: section %s shrank from %d to %d rows before the final value", i, id, beforeRows[id], afterRows[id])
			}
		}
	}
}
