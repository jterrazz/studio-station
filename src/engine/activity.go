package engine

import (
	"context"
	"sort"

	"github.com/jterrazz/studio-station/src/activity"

	model "github.com/jterrazz/studio-station/src/machines"
)

const (
	NoticeActivity = "activity"
	EventDiskLow   = "disk_low"
)

type Tick func(ctx context.Context)

func (d *Daemon) History() *activity.History {
	// value: the activity this engine keeps of every machine it sees, since
	// it started; nil before Begin.
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.history
}

type Running struct {
	Name     string
	Lifetime model.Lifetime
	// value: the process that holds its guest on this Mac, 0 while none does.
	PID int
	// value: station's guest answers inside it.
	Guest bool
}

func (d *Daemon) Running() []Running {
	d.mu.Lock()
	defer d.mu.Unlock()
	var out []Running
	for name, v := range d.vms {
		if v.state != StateRunning {
			continue
		}
		out = append(out, Running{Name: name, Lifetime: v.rec.Lifetime, PID: v.holder(), Guest: v.guest == GuestConnected})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func (d *Daemon) Tell(machine, finding, text string) {
	// invariant: told once, to the open surfaces and to the owner's
	// notifications, as every word the engine says by itself; a disk under
	// its floor is the event of SPEC §10.4, any other finding the activity's.
	kind := NoticeActivity
	if finding == activity.SubjectDisk {
		kind = EventDiskLow
	}
	d.hub.tell(Notice{Kind: kind, Text: text})
	d.notify(context.Background(), Notification{Kind: kind, Subject: machine, Text: text})
}
