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

type slowInventory struct {
	release chan struct{}
}

func (i slowInventory) Installed(ctx context.Context, each func(map[string]bool)) {
	each(map[string]bool{"quick": true})
	select {
	case <-i.release:
		each(map[string]bool{"quick": true, "late": true})
	case <-ctx.Done():
	}
}

func TestASizedRowShowsItsSizeBeforeASlowInventoryAnswers(t *testing.T) {
	fs := systemtest.NewFS()
	fs.Files["h/quick/tree/f"] = &fstest.MapFile{Data: []byte("abc")}
	fs.Files["h/late/tree/f"] = &fstest.MapFile{Data: []byte("abcde")}
	tools := []catalog.Tool{
		{Name: "quick", Installs: catalog.HostOnly(catalog.Brew("quick")), Storage: catalog.Storage{Paths: []catalog.Stored{{Path: "~/quick/*", Level: catalog.LevelCache}}}},
		{Name: "late", Installs: catalog.HostOnly(catalog.Brew("late")), Storage: catalog.Storage{Paths: []catalog.Stored{{Path: "~/late/*", Level: catalog.LevelCache}}}},
	}
	inventory := slowInventory{release: make(chan struct{})}
	s := New(system.Host{FS: fs, Clock: realClock{}}, tools, Options{Disk: systemtest.Disk{FS: fs}, Here: catalog.Host, Expand: Expander{Home: "/h"}, Inventory: inventory})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	reports := s.Watch(ctx)
	var rows map[string]Row
	for r := range reports {
		rows = map[string]Row{}
		for _, row := range r.Rows {
			rows[row.Entry.Name] = row
		}
		if quick, late := rows["quick"], rows["late"]; !quick.Pending && !late.Pending && quick.State == catalog.StateOK {
			break
		}
	}
	if ctx.Err() != nil {
		t.Fatalf("no row was sized while the inventory had not answered: %+v", rows)
	}
	if quick := rows["quick"]; quick.Bytes[catalog.LevelCache] != 3 {
		t.Errorf("the row whose tool answered is %+v, want its 3 bytes and its mark", quick)
	}
	if late := rows["late"]; late.Bytes[catalog.LevelCache] != 5 || late.State != catalog.StatePending {
		t.Errorf("the row whose tool has not answered is %+v, want its 5 bytes and a pending mark", late)
	}

	close(inventory.release)
	var last Report
	for r := range reports {
		last = r
	}
	for _, row := range last.Rows {
		if row.Entry.Name == "late" && row.State != catalog.StateOK {
			t.Errorf("once the inventory answered the late row is %+v, want ok", row)
		}
	}
}
