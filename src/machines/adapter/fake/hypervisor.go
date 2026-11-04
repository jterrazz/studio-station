package fake

import (
	"context"
	"errors"
	"net/netip"
	"sync"
	"time"

	"github.com/jterrazz/studio-station/src/core/call"
	imagescontract "github.com/jterrazz/studio-station/src/images/contract"
	"github.com/jterrazz/studio-station/src/machines"
	"github.com/jterrazz/studio-station/src/machines/contract"
)

var (
	ErrNoDisk   = errors.New("no such VM disk")
	ErrNoImage  = errors.New("no such image")
	ErrNotBoot  = errors.New("the VM does not run")
	ErrTwoRuns  = errors.New("the VM already runs")
	ErrHeldName = errors.New("a VM disk already has this name")
)

type disk struct {
	size machines.Size
	run  *run
}

type Hypervisor struct {
	mu     sync.Mutex
	name   string
	images map[string]bool
	disks  map[string]*disk
	pids   int
}

func New(name string) *Hypervisor {
	return &Hypervisor{name: name, images: map[string]bool{}, disks: map[string]*disk{}}
}

func (h *Hypervisor) Name() string    { return h.name }
func (h *Hypervisor) Installed() bool { return true }

func (h *Hypervisor) Version(context.Context) (string, error) { return "1.0.0", nil }

func (h *Hypervisor) Disks(context.Context) ([]contract.Disk, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]contract.Disk, 0, len(h.disks))
	for vm, d := range h.disks {
		state := contract.DiskStopped
		if d.run != nil {
			state = contract.DiskRunning
		}
		out = append(out, contract.Disk{VM: vm, Bytes: d.size.Disk, State: state})
	}
	return out, nil
}

func (h *Hypervisor) Fetch(_ context.Context, img imagescontract.ImageLocation, p call.Progress) error {
	if p != nil {
		p.Step("fetching", img.Ref)
		defer p.Done("fetching", nil)
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.images[img.Ref] = true
	return nil
}

func (h *Hypervisor) Clone(_ context.Context, from imagescontract.ImageLocation, vm string) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if !h.images[from.Ref] && h.disks[from.Ref] == nil {
		return ErrNoImage
	}
	if h.disks[vm] != nil {
		return ErrHeldName
	}
	h.disks[vm] = &disk{}
	return nil
}

func (h *Hypervisor) Resize(_ context.Context, vm string, s machines.Size) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	d, ok := h.disks[vm]
	if !ok {
		return ErrNoDisk
	}
	d.size = s
	return nil
}

func (h *Hypervisor) Rename(_ context.Context, from, to string) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	d, ok := h.disks[from]
	if !ok {
		return ErrNoDisk
	}
	if h.disks[to] != nil {
		return ErrHeldName
	}
	delete(h.disks, from)
	h.disks[to] = d
	return nil
}

func (h *Hypervisor) Boot(_ context.Context, vm string, _ contract.BootOptions) (contract.Run, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	d, ok := h.disks[vm]
	if !ok {
		return nil, ErrNoDisk
	}
	if d.run != nil {
		return nil, ErrTwoRuns
	}
	h.pids++
	d.run = &run{vm: vm, pid: 1000 + h.pids, ended: make(chan machines.RunEnd, 1)}
	return d.run, nil
}

func (h *Hypervisor) Adopt(_ context.Context, vm string) (contract.Run, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if d, ok := h.disks[vm]; ok && d.run != nil {
		return d.run, true
	}
	return nil, false
}

func (h *Hypervisor) End(vm string, end machines.RunEnd) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.end(vm, end)
}

func (h *Hypervisor) end(vm string, end machines.RunEnd) {
	d, ok := h.disks[vm]
	if !ok || d.run == nil {
		return
	}
	d.run.ended <- end
	close(d.run.ended)
	d.run = nil
}

func (h *Hypervisor) Shutdown(_ context.Context, vm string, _ time.Duration) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.disks[vm]; !ok {
		return ErrNoDisk
	}
	h.end(vm, machines.RunEnd{Reason: machines.GuestPoweroff})
	return nil
}

func (h *Hypervisor) Delete(_ context.Context, vm string) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.disks[vm]; !ok {
		return ErrNoDisk
	}
	h.end(vm, machines.RunEnd{Reason: machines.GuestPoweroff})
	delete(h.disks, vm)
	return nil
}

func (h *Hypervisor) Discard(_ context.Context, img imagescontract.ImageLocation) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.images, img.Ref)
	delete(h.disks, img.Ref)
	return nil
}

func (h *Hypervisor) Address(_ context.Context, vm string, _ time.Duration) (netip.Addr, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	d, ok := h.disks[vm]
	if !ok || d.run == nil {
		return netip.Addr{}, ErrNotBoot
	}
	return netip.MustParseAddr("192.168.64.5"), nil
}

type run struct {
	vm    string
	pid   int
	ended chan machines.RunEnd
}

func (r *run) VM() string                    { return r.vm }
func (r *run) PID() int                      { return r.pid }
func (r *run) Ended() <-chan machines.RunEnd { return r.ended }
