package process

import (
	"context"
	"maps"
	"slices"
	"sort"
	"time"

	model "github.com/jterrazz/studio-station/src/machines"

	"github.com/jterrazz/studio-station/src/engine"
	"github.com/jterrazz/studio-station/src/ports"
)

const (
	exposureEvery = 5 * time.Minute
)

func (d *Daemon) ExposureKept() engine.ExposureKept {
	d.mu.Lock()
	defer d.mu.Unlock()
	return engine.ExposureKept{At: d.exposedAt, Scanned: maps.Clone(d.scanned), Red: slices.Sorted(slices.Values(d.red))}
}

func (d *Daemon) RememberExposure(k engine.ExposureKept) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.exposedAt, d.scanned, d.red = k.At, maps.Clone(k.Scanned), slices.Clone(k.Red)
}

func (d *Daemon) hear(vm string, ls []ports.Listener) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.heard[vm] = ls
}

// value: whether a guest reported listeners other than the job last read;
// the caller holds d.mu.
func (d *Daemon) reheardLocked() bool {
	for vm, ls := range d.heard {
		if !slices.Equal(d.scanned[vm], ls) {
			return true
		}
	}
	return false
}

func (d *Daemon) heardOf(vm string) []ports.Listener {
	d.mu.Lock()
	defer d.mu.Unlock()
	return slices.Clone(d.heard[vm])
}

// value: what listens in each running VM, placed behind its Mac or bridged
// onto its network; the caller holds d.mu.
func (d *Daemon) listeningLocked() []ports.Machine {
	names := make([]string, 0, len(d.heard))
	for vm := range d.heard {
		names = append(names, vm)
	}
	sort.Strings(names)
	out := make([]ports.Machine, 0, len(names))
	for _, vm := range names {
		v := d.vms[vm]
		out = append(out, ports.VMOf(vm, d.m.HereName(), v != nil && v.rec.Network == model.Bridged, slices.Clone(d.heard[vm])))
	}
	return out
}

// invariant: the exposure job (SPEC 5.6.1): this Mac's listeners and its
// VMs' are read at the start, every exposureEvery and on every listener
// change a guest reports; a port that turns red is told once, and again
// only after it stopped being red.
func (d *Daemon) watchExposure(ctx context.Context) {
	if d.o.Listening == nil {
		return
	}
	now := d.h.Now()
	d.mu.Lock()
	if !d.reheardLocked() && !d.exposedAt.IsZero() && now.Sub(d.exposedAt) < exposureEvery {
		d.mu.Unlock()
		return
	}
	d.exposedAt, d.scanned = now, maps.Clone(d.heard)
	vms := d.listeningLocked()
	d.mu.Unlock()
	mine, _ := d.o.Listening(ctx)
	var decl []ports.Declaration
	if d.o.Declarations != nil {
		decl, _ = d.o.Declarations(ctx)
	}
	found := ports.Exposed(ports.Survey(append([]ports.Machine{ports.MacOf(d.m.HereName(), mine)}, vms...), decl))
	d.mu.Lock()
	was := d.red
	d.red = make([]string, 0, len(found))
	var fresh []string
	for _, f := range found {
		d.red = append(d.red, f.Subject)
		if !slices.Contains(was, f.Subject) {
			fresh = append(fresh, ports.Told(f))
		}
	}
	d.mu.Unlock()
	for _, text := range fresh {
		d.tell(ctx, d.m.HereName(), engine.Told{Kind: engine.EventExposed, Parts: []string{text}})
	}
}
