package engine

import (
	"context"
	"sort"
	"time"
)

const (
	NoticeRecalled = "lend_recalled"
	lendEvery      = time.Minute
	lendRetell     = time.Hour
)

type Lend struct {
	Name  string
	Until time.Time
}

type Lends interface {
	Held() (map[string][]Lend, error)
	Recall(ctx context.Context, from string, names []string) error
}

// invariant: the records are read at the start and every lendEvery; a lend
// past its until on a running VM is recalled by the machinery of `secrets
// recall`. A failed recall keeps its entry and is logged once per lendRetell.
func (d *Daemon) lendsDue() bool {
	now := d.h.Clock.Now()
	d.mu.Lock()
	defer d.mu.Unlock()
	if !d.lendsAt.IsZero() && now.Sub(d.lendsAt) < lendEvery {
		return false
	}
	d.lendsAt = now
	return true
}

func (d *Daemon) sweepLends(ctx context.Context) {
	if d.o.Lends == nil || !d.lendsDue() {
		return
	}
	go func() {
		if !d.lendMu.TryLock() {
			return
		}
		defer d.lendMu.Unlock()
		d.recallExpired(ctx, "")
	}()
}

// invariant: a lasting VM stopped past a lend's until has that lend recalled
// at its next start, before its guest is reported connected.
func (d *Daemon) recallBeforeReady(ctx context.Context, vm string) {
	if d.o.Lends == nil {
		return
	}
	d.lendMu.Lock()
	defer d.lendMu.Unlock()
	d.recallExpired(ctx, vm)
}

func (d *Daemon) recallExpired(ctx context.Context, only string) {
	held, err := d.o.Lends.Held()
	if err != nil {
		return
	}
	now := d.h.Clock.Now()
	vms := make([]string, 0, len(held))
	for vm := range held {
		vms = append(vms, vm)
	}
	sort.Strings(vms)
	for _, vm := range vms {
		if only != "" && vm != only || only == "" && !d.isConnected(vm) {
			continue
		}
		var names []string
		for _, l := range held[vm] {
			if !now.Before(l.Until) {
				names = append(names, l.Name)
			}
		}
		if len(names) > 0 {
			d.recall(ctx, vm, names)
		}
	}
}

// invariant: the sweep recalls only through a connected guest; a VM still
// connecting has its expired lends recalled as it connects, before it is ready.
func (d *Daemon) isConnected(vm string) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	v := d.vms[vm]
	return v != nil && v.state == StateRunning && v.guest == GuestConnected
}

func (d *Daemon) recall(ctx context.Context, vm string, names []string) {
	err := d.o.Lends.Recall(ctx, vm, names)
	now := d.h.Clock.Now()
	for _, n := range names {
		key := vm + " " + n
		if err == nil {
			d.mu.Lock()
			delete(d.recallFailed, key)
			d.mu.Unlock()
			d.notify(ctx, Notification{Kind: NoticeRecalled, Subject: vm + " lend", Text: "recalled " + n + " (expired)"})
			d.hub.tell(Notice{Kind: NoticeRecalled, Text: n + " recalled from " + vm + " (expired)"})
			continue
		}
		d.mu.Lock()
		last, told := d.recallFailed[key]
		retell := !told || now.Sub(last) >= lendRetell
		if retell {
			d.recallFailed[key] = now
		}
		d.mu.Unlock()
		if retell {
			d.notify(ctx, Notification{Kind: NoticeRecallFailed, Subject: vm + " lend", Text: "recall of " + n + " failed: " + err.Error()})
		}
	}
}
