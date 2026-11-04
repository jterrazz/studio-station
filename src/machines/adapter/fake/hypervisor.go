package fake

import (
	"context"
	"errors"
	"maps"
	"net/netip"
	"slices"
	"sync"
	"time"

	"github.com/jterrazz/studio-station/src/core/call"
	imagescontract "github.com/jterrazz/studio-station/src/images/contract"
	"github.com/jterrazz/studio-station/src/machines"
	"github.com/jterrazz/studio-station/src/machines/contract"
	"github.com/jterrazz/studio-station/src/world"
)

var (
	ErrNoDisk   = errors.New("no such VM disk")
	ErrNoImage  = errors.New("no such image")
	ErrNotBoot  = errors.New("the VM does not run")
	ErrTwoRuns  = errors.New("the VM already runs")
	ErrHeldName = errors.New("a VM disk already has this name")
)

type Hypervisor struct {
	// invariant: the hypervisor of one Mac of the world: its disks and runs are
	// that Mac's VMs, so a run outlives the adapter, as a VM outlives its engine,
	// and an event of the world ends it.
	w    *world.World
	mac  string
	name string
}

func HypervisorOf(w *world.World, mac, name string) *Hypervisor {
	return &Hypervisor{w: w, mac: mac, name: name}
}

func (h *Hypervisor) Name() string    { return h.name }
func (h *Hypervisor) Installed() bool { return true }

func (h *Hypervisor) Version(context.Context) (string, error) { return "1.0.0", nil }

func (h *Hypervisor) here() *world.Mac { return h.w.Mac(h.mac) }

func (h *Hypervisor) disk(vm string) (*world.VM, bool) {
	v, ok := h.here().VMs[vm]
	return v, ok && v.Disk
}

func (h *Hypervisor) Disks(context.Context) ([]contract.Disk, error) {
	h.w.Lock()
	defer h.w.Unlock()
	out := []contract.Disk{}
	for _, vm := range slices.Sorted(maps.Keys(h.here().VMs)) {
		v := h.here().VMs[vm]
		if !v.Disk {
			continue
		}
		state := contract.DiskStopped
		if v.Running() {
			state = contract.DiskRunning
		}
		out = append(out, contract.Disk{VM: vm, Bytes: v.Size.Disk, State: state})
	}
	return out, nil
}

func (h *Hypervisor) Fetch(_ context.Context, img imagescontract.ImageLocation, p call.Progress) error {
	if p != nil {
		p.Step("fetching", img.Ref)
		defer p.Done("fetching", nil)
	}
	h.w.Lock()
	defer h.w.Unlock()
	if m := h.here(); !slices.Contains(m.Images, img.Ref) {
		m.Images = append(m.Images, img.Ref)
	}
	return nil
}

func (h *Hypervisor) Clone(_ context.Context, from imagescontract.ImageLocation, vm string) error {
	h.w.Lock()
	defer h.w.Unlock()
	m := h.here()
	if _, ok := h.disk(from.Ref); !ok && !slices.Contains(m.Images, from.Ref) {
		return ErrNoImage
	}
	if _, ok := h.disk(vm); ok {
		return ErrHeldName
	}
	v := m.VMs[vm]
	if v == nil {
		v = &world.VM{}
		m.VMs[vm] = v
	}
	v.Disk = true
	return nil
}

func (h *Hypervisor) Resize(_ context.Context, vm string, s machines.Size) error {
	h.w.Lock()
	defer h.w.Unlock()
	v, ok := h.disk(vm)
	if !ok {
		return ErrNoDisk
	}
	v.Size = s
	return nil
}

func (h *Hypervisor) Rename(_ context.Context, from, to string) error {
	h.w.Lock()
	defer h.w.Unlock()
	v, ok := h.disk(from)
	if !ok {
		return ErrNoDisk
	}
	if _, held := h.disk(to); held {
		return ErrHeldName
	}
	m := h.here()
	delete(m.VMs, from)
	if kept := m.VMs[to]; kept != nil {
		v.Kept = kept.Kept
	}
	m.VMs[to] = v
	return nil
}

func (h *Hypervisor) Boot(_ context.Context, vm string, _ contract.BootOptions) (contract.Run, error) {
	h.w.Lock()
	defer h.w.Unlock()
	v, ok := h.disk(vm)
	if !ok {
		return nil, ErrNoDisk
	}
	if v.Running() {
		return nil, ErrTwoRuns
	}
	v.PID, v.Ended = h.nextPID(), nil
	return &run{h: h, vm: vm, pid: v.PID}, nil
}

// value: pids count up over the whole world, so no two runs share one.
func (h *Hypervisor) nextPID() int {
	top := 1000
	for _, m := range h.w.Macs {
		for _, v := range m.VMs {
			top = max(top, v.PID)
		}
	}
	return top + 1
}

func (h *Hypervisor) Adopt(_ context.Context, vm string) (contract.Run, bool) {
	h.w.Lock()
	defer h.w.Unlock()
	if v, ok := h.disk(vm); ok && v.Running() {
		return &run{h: h, vm: vm, pid: v.PID}, true
	}
	return nil, false
}

func End(r contract.Run, end machines.RunEnd) {
	// value: ends r's run the way its guest would, wherever the run was adopted.
	if r, ok := r.(*run); ok {
		r.h.end(r.vm, r.pid, end)
	}
}

func (h *Hypervisor) end(vm string, pid int, end machines.RunEnd) {
	h.w.Lock()
	defer h.w.Unlock()
	if v, ok := h.disk(vm); ok && v.PID == pid {
		v.PID, v.Ended = 0, &end
	}
}

func (h *Hypervisor) Shutdown(_ context.Context, vm string, _ time.Duration) error {
	h.w.Lock()
	defer h.w.Unlock()
	v, ok := h.disk(vm)
	if !ok {
		return ErrNoDisk
	}
	if v.Running() {
		v.PID, v.Ended = 0, &machines.RunEnd{Reason: machines.GuestPoweroff}
	}
	return nil
}

func (h *Hypervisor) Delete(_ context.Context, vm string) error {
	h.w.Lock()
	defer h.w.Unlock()
	v, ok := h.disk(vm)
	if !ok {
		return ErrNoDisk
	}
	v.Disk, v.PID, v.Ended = false, 0, nil
	if v.Kept == nil {
		delete(h.here().VMs, vm)
	}
	return nil
}

func (h *Hypervisor) Discard(_ context.Context, img imagescontract.ImageLocation) error {
	h.w.Lock()
	defer h.w.Unlock()
	m := h.here()
	m.Images = slices.DeleteFunc(m.Images, func(ref string) bool { return ref == img.Ref })
	if v, ok := m.VMs[img.Ref]; ok && v.Kept == nil {
		delete(m.VMs, img.Ref)
	}
	return nil
}

func (h *Hypervisor) Address(_ context.Context, vm string, _ time.Duration) (netip.Addr, error) {
	h.w.Lock()
	defer h.w.Unlock()
	v, ok := h.disk(vm)
	if !ok || !v.Running() {
		return netip.Addr{}, ErrNotBoot
	}
	if v.Address != "" {
		return netip.ParseAddr(v.Address)
	}
	return netip.MustParseAddr("192.168.64.5"), nil
}

// invariant: a run is over once its VM no longer runs under its pid; how it
// ended is read from the world, once, then kept.
type run struct {
	h   *Hypervisor
	vm  string
	pid int

	once sync.Mutex
	end  *machines.RunEnd
}

func (r *run) VM() string { return r.vm }
func (r *run) PID() int   { return r.pid }

func (r *run) Ended() <-chan machines.RunEnd {
	r.once.Lock()
	defer r.once.Unlock()
	if r.end == nil {
		r.h.w.Lock()
		v, ok := r.h.here().VMs[r.vm]
		switch {
		case ok && v.PID == r.pid:
		case ok && v.Ended != nil:
			r.end, v.Ended = v.Ended, nil
		default:
			r.end = &machines.RunEnd{Reason: machines.Unknown, ExitCode: -1}
		}
		r.h.w.Unlock()
	}
	ch := make(chan machines.RunEnd, 1)
	if r.end != nil {
		ch <- *r.end
		close(ch)
	}
	return ch
}
