package engine

import (
	"context"
	"slices"
	"sync"
)

const (
	// value: the machine a watch of activity means when it names none: This
	// Mac, the engine's own.
	ThisMac = ""
)

type watchers struct {
	mu sync.Mutex
	on map[string]int
}

func (w *watchers) hold(machines []string, by int) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.on == nil {
		w.on = map[string]int{}
	}
	for _, m := range machines {
		w.on[m] += by
		if w.on[m] <= 0 {
			delete(w.on, m)
		}
	}
}

func (w *watchers) watched(machine string) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.on[machine] > 0
}

type Watching struct {
	Machines []string `json:"machines"`
}

func (d *Daemon) Watch(ctx context.Context, machines []string, send func(any) error) error {
	// invariant: SPEC §6.3: a machine is watched while a surface holds this
	// watch open on it (the menu bar open, a terminal UI on its Activity),
	// and only then; a watch of pages or events alone, which the menu bar
	// holds from its launch, never makes one.
	held := slices.Compact(slices.Sorted(slices.Values(machines)))
	if len(held) == 0 {
		held = []string{ThisMac}
	}
	d.watchers.hold(held, 1)
	defer d.watchers.hold(held, -1)
	if err := send(Watching{Machines: slices.DeleteFunc(slices.Clone(held), func(m string) bool { return m == ThisMac })}); err != nil {
		return nil
	}
	<-ctx.Done()
	return nil
}

func (d *Daemon) Watched(machine string) bool { return d.watchers.watched(machine) }
