package engine

import (
	"slices"
	"sort"

	"github.com/jterrazz/studio-station/src/domain/machines"
	"github.com/jterrazz/studio-station/src/ports"
)

func (d *Daemon) hear(vm string, ls []ports.Listener) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.heard[vm] = ls
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
		out = append(out, ports.VMOf(vm, d.m.HereName(), v != nil && v.rec.Network == machines.Bridged, slices.Clone(d.heard[vm])))
	}
	return out
}
