package fake

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"strconv"
	"strings"
	"sync"

	"github.com/jterrazz/studio-station/src/machines"
	"github.com/jterrazz/studio-station/src/machines/contract"
	"github.com/jterrazz/studio-station/src/world"
)

var (
	ErrNoGuest   = errors.New("no guest answers at this address")
	ErrClosed    = errors.New("the guest connection is closed")
	ErrNoService = errors.New("nothing listens on this guest port")
)

type Answer struct {
	Code           int
	Stdout, Stderr string
}

type File struct {
	Data []byte
	Mode fs.FileMode
}

type VM struct {
	mu      sync.Mutex
	answers map[string]Answer
	// value: the lines that power the VM off once answered.
	poweroffs map[string]bool
	received  map[string][]byte
	files     map[string]File
	services  map[int]string
}

func NewVM() *VM {
	return &VM{answers: map[string]Answer{}, poweroffs: map[string]bool{}, received: map[string][]byte{}, files: map[string]File{}, services: map[int]string{}}
}

func (v *VM) Program(line string, a Answer) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.answers[line] = a
}

func (v *VM) Serve(port int, greeting string) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.services[port] = greeting
}

func (v *VM) Received(line string) []byte {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.received[line]
}

func (v *VM) File(path string) (File, bool) {
	v.mu.Lock()
	defer v.mu.Unlock()
	f, ok := v.files[path]
	return f, ok
}

type Guests struct {
	// invariant: the guests of the world's VMs: a VM whose slice has a guest
	// answers what it says, one with none is not reached.
	w   *world.World
	mu  sync.Mutex
	vms map[string]*VM
}

func GuestsOf(w *world.World) *Guests { return &Guests{w: w, vms: map[string]*VM{}} }

func NewGuests() *Guests { return GuestsOf(&world.World{Here: "here"}) }

func (g *Guests) Add(vm string) *VM {
	g.w.Lock()
	if g.w.Holder(vm) == "" {
		g.w.Mac(g.w.Here).VMs[vm] = &world.VM{}
	}
	g.w.Macs[g.w.Holder(vm)].VMs[vm].Guest = &world.Guest{}
	g.w.Unlock()
	g.mu.Lock()
	defer g.mu.Unlock()
	v := NewVM()
	g.vms[vm] = v
	return v
}

func (g *Guests) answering(vm string) (map[string]world.Answer, bool) {
	g.w.Lock()
	defer g.w.Unlock()
	mac := g.w.Holder(vm)
	if mac == "" || g.w.Macs[mac].VMs[vm].Guest == nil {
		return nil, false
	}
	return g.w.Macs[mac].VMs[vm].Guest.Answers, true
}

func (g *Guests) Dial(_ context.Context, t contract.GuestTarget) (contract.Guest, error) {
	answers, ok := g.answering(t.VM)
	if !ok {
		return nil, ErrNoGuest
	}
	g.mu.Lock()
	v, ok := g.vms[t.VM]
	if !ok {
		v = NewVM()
		for line, a := range answers {
			v.answers[line] = Answer{Code: a.Code, Stdout: a.Stdout, Stderr: a.Stderr}
			v.poweroffs[line] = a.Poweroff
		}
		g.vms[t.VM] = v
	}
	g.mu.Unlock()
	return &guest{vm: v, name: t.VM, w: g.w, done: make(chan struct{})}, nil
}

type guest struct {
	vm   *VM
	name string
	w    *world.World
	done chan struct{}
	once sync.Once
}

var ErrGuestBroke = errors.New("the guest connection broke, as the world scripted it")

func (v *VM) answer(line string) (a Answer, poweroff, ok bool) {
	if a, ok := v.answers[line]; ok {
		return a, v.poweroffs[line], true
	}
	for pattern, a := range v.answers {
		if prefix, wild := strings.CutSuffix(pattern, "*"); wild && strings.HasPrefix(line, prefix) {
			return a, v.poweroffs[pattern], true
		}
	}
	return Answer{}, false, false
}

// invariant: a scripted Guest.Exec fault breaks a command the guest answers;
// a line it does not know, as the engine's station session to a world VM, is
// "not found" and spends no fault.
func (c *guest) fails() bool {
	c.w.Lock()
	defer c.w.Unlock()
	return c.w.Fails("Guest.Exec")
}

// invariant: a guest that powers its VM off ends the VM's run as a guest
// poweroff, and the connection with it.
func (c *guest) poweroff() {
	c.w.Lock()
	if mac := c.w.Holder(c.name); mac != "" {
		if v := c.w.Macs[mac].VMs[c.name]; v.Running() {
			v.PID, v.Ended = 0, &machines.RunEnd{Reason: machines.GuestPoweroff}
		}
	}
	c.w.Unlock()
	_ = c.Close()
}

func (c *guest) closed() bool {
	select {
	case <-c.done:
		return true
	default:
		return false
	}
}

func (c *guest) Exec(_ context.Context, cmd contract.GuestCommand) (contract.GuestResult, error) {
	if c.closed() {
		return contract.GuestResult{}, ErrClosed
	}
	var in []byte
	if cmd.Stdin != nil {
		var err error
		if in, err = io.ReadAll(cmd.Stdin); err != nil {
			return contract.GuestResult{}, err
		}
	}
	c.vm.mu.Lock()
	a, poweroff, ok := c.vm.answer(cmd.Line)
	c.vm.mu.Unlock()
	switch {
	case !ok:
		a = Answer{Code: 127, Stderr: cmd.Line + ": not found\n"}
	case c.fails():
		return contract.GuestResult{}, ErrGuestBroke
	default:
		c.vm.mu.Lock()
		c.vm.received[cmd.Line] = in
		c.vm.mu.Unlock()
	}
	res := contract.GuestResult{Code: a.Code, Stdout: a.Stdout, Stderr: a.Stderr}
	if cmd.Stdout != nil {
		if _, err := cmd.Stdout.Write([]byte(a.Stdout)); err != nil {
			return contract.GuestResult{}, err
		}
		res.Stdout = ""
	}
	if cmd.Stderr != nil {
		if _, err := cmd.Stderr.Write([]byte(a.Stderr)); err != nil {
			return contract.GuestResult{}, err
		}
		res.Stderr = ""
	}
	if poweroff {
		c.poweroff()
	}
	return res, nil
}

func (c *guest) Forward(_ context.Context, _ string, port int) (io.ReadWriteCloser, error) {
	if c.closed() {
		return nil, ErrClosed
	}
	c.vm.mu.Lock()
	greeting, ok := c.vm.services[port]
	c.vm.mu.Unlock()
	if !ok {
		return nil, errors.Join(ErrNoService, errors.New("port "+strconv.Itoa(port)))
	}
	return &stream{Reader: strings.NewReader(greeting + "\n")}, nil
}

type stream struct{ *strings.Reader }

func (s *stream) Write(p []byte) (int, error) { return len(p), nil }
func (s *stream) Close() error                { return nil }

func (c *guest) Put(_ context.Context, path string, data []byte, mode fs.FileMode) error {
	if c.closed() {
		return ErrClosed
	}
	c.vm.mu.Lock()
	defer c.vm.mu.Unlock()
	c.vm.files[path] = File{Data: append([]byte(nil), data...), Mode: mode}
	return nil
}

func (c *guest) Close() error {
	c.once.Do(func() { close(c.done) })
	return nil
}

func (c *guest) Done() <-chan struct{} { return c.done }
