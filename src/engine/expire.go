package engine

import (
	"context"
	"sort"
	"time"

	"github.com/jterrazz/studio-station/src/accounts"
)

const (
	expireEvery  = time.Minute
	expireRetell = time.Hour
)

type Lends interface {
	// value: the account shares of this Mac, by VM; Expire deletes a VM's
	// copies and their shares, Forget the shares of a VM that is gone.
	Held() (map[string][]accounts.Share, error)
	Expire(ctx context.Context, vm string, names []string) error
	Forget(ctx context.Context, vm string) error
}

// invariant: the expire job: the shares are read at the start and every
// expireEvery; a share past its until on a running VM is taken back, with no
// notification. A take-back that fails keeps its share and is told once per
// expireRetell.
func (d *Engine) lendsDue() bool {
	now := d.h.Clock.Now()
	d.mu.Lock()
	defer d.mu.Unlock()
	if !d.lendsAt.IsZero() && now.Sub(d.lendsAt) < expireEvery {
		return false
	}
	d.lendsAt = now
	return true
}

func (d *Engine) sweepLends(ctx context.Context) {
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

// invariant: a VM stopped past a share's until has it taken back at its
// next start, before its guest is reported connected.
func (d *Engine) recallBeforeReady(ctx context.Context, vm string) {
	if d.o.Lends == nil {
		return
	}
	d.lendMu.Lock()
	defer d.lendMu.Unlock()
	d.recallExpired(ctx, vm)
}

func (d *Engine) recallExpired(ctx context.Context, only string) {
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
		for _, s := range accounts.Expired(held[vm], now) {
			names = append(names, s.Name)
		}
		if len(names) > 0 {
			d.recall(ctx, vm, names)
		}
	}
}

// invariant: the sweep takes back only through a connected guest; a VM still
// connecting has its ended shares taken back as it connects, before it is ready.
func (d *Engine) isConnected(vm string) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	v := d.vms[vm]
	return v != nil && v.state == StateRunning && v.guest == GuestConnected
}

func (d *Engine) recall(ctx context.Context, vm string, names []string) {
	err := d.o.Lends.Expire(ctx, vm, names)
	now := d.h.Clock.Now()
	for _, n := range names {
		key := vm + " " + n
		d.mu.Lock()
		if err == nil {
			delete(d.recallFailed, key)
			d.mu.Unlock()
			continue
		}
		last, told := d.recallFailed[key]
		retell := !told || now.Sub(last) >= expireRetell
		if retell {
			d.recallFailed[key] = now
		}
		d.mu.Unlock()
		if retell {
			d.notify(ctx, Notification{Kind: NoticeRecallFailed, Subject: vm + " share", Text: "taking " + accounts.Label(accounts.AccountOf(n)) + " back failed: " + err.Error()})
		}
	}
}

// invariant: a VM deleted takes its copies with its disk; its shares go
// with it.
func (d *Engine) forgetShares(ctx context.Context, vm string) {
	if d.o.Lends != nil {
		_ = d.o.Lends.Forget(ctx, vm)
	}
}
