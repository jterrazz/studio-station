package engine

import (
	"context"
	"sort"

	"github.com/jterrazz/studio-station/src/activity"
	"github.com/jterrazz/studio-station/src/domain/machines"
)

const NoticeActivity = "activity"

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
	Lifetime machines.Lifetime
	PID      int
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
		out = append(out, Running{Name: name, Lifetime: v.rec.Lifetime, PID: v.runPID(), Guest: v.guest == GuestConnected})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func (d *Daemon) Tell(subject, text string) {
	// invariant: told once, to the open surfaces and to the owner's
	// notifications, as every word the engine says by itself.
	d.hub.tell(Notice{Kind: NoticeActivity, Text: text})
	d.notify(context.Background(), Notification{Kind: NoticeActivity, Subject: subject, Text: text})
}
