package fake

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"strconv"
	"strings"
	"sync"

	"github.com/jterrazz/studio-station/src/machines/contract"
	"github.com/jterrazz/studio-station/src/world"
)

var (
	ErrNoGuest   = errors.New("no guest answers at this address")
	ErrClosed    = errors.New("the guest connection is closed")
	ErrNoService = errors.New("nothing listens on this guest port")
	ErrRefused   = errors.New("the guest connection is refused, as the world scripted it")
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
	mu       sync.Mutex
	answers  map[string]Answer
	received map[string][]byte
	files    map[string]File
	services map[int]string
}

func NewVM() *VM {
	return &VM{answers: map[string]Answer{}, received: map[string][]byte{}, files: map[string]File{}, services: map[int]string{}}
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
	// value: the line station's own session runs on in a guest.
	session string
}

func (g *Guests) Serving(session string) *Guests {
	g.session = session
	return g
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

func (g *Guests) answering(vm string) (*world.Guest, bool) {
	g.w.Lock()
	defer g.w.Unlock()
	mac := g.w.Holder(vm)
	if mac == "" || g.w.Macs[mac].VMs[vm].Guest == nil {
		return nil, false
	}
	return g.w.Macs[mac].VMs[vm].Guest, true
}

func (g *Guests) Dial(_ context.Context, t contract.GuestTarget) (contract.Guest, error) {
	g.w.Lock()
	refused := g.w.Fails("Guest.Dial")
	g.w.Unlock()
	if refused {
		return nil, ErrRefused
	}
	wg, ok := g.answering(t.VM)
	if !ok {
		return nil, ErrNoGuest
	}
	answers := wg.Answers
	g.mu.Lock()
	v, ok := g.vms[t.VM]
	if !ok {
		v = NewVM()
		for line, a := range answers {
			v.answers[line] = Answer{Code: a.Code, Stdout: a.Stdout, Stderr: a.Stderr}
		}
		g.vms[t.VM] = v
	}
	g.mu.Unlock()
	serving := g.session
	if wg.Replaced {
		serving = ""
	}
	return &guest{vm: v, serving: serving, fresh: wg.Fresh, done: make(chan struct{})}, nil
}

type guest struct {
	vm *VM
	// value: the line of station's own session, which this guest runs as its
	// image installs it; empty when its tooling replaced station's guest.
	serving string
	// value: a guest made from station's image: a line no answer scripts
	// succeeds, as the image's own commands do.
	fresh bool
	done  chan struct{}
	once  sync.Once
}

// invariant: station's session in the fake guest says it is ready, takes
// whatever the engine writes, and lasts until the connection closes.
func (c *guest) serve(ctx context.Context, cmd contract.GuestCommand) (contract.GuestResult, error) {
	if cmd.Stdin != nil {
		go func() {
			buf := make([]byte, 4096)
			for {
				if _, err := cmd.Stdin.Read(buf); err != nil {
					return
				}
			}
		}()
	}
	if cmd.Stdout != nil {
		if _, err := cmd.Stdout.Write([]byte(`{"v":1,"type":"ready"}` + "\n")); err != nil {
			return contract.GuestResult{}, err
		}
	}
	select {
	case <-ctx.Done():
	case <-c.done:
	}
	return contract.GuestResult{}, nil
}

func (c *guest) closed() bool {
	select {
	case <-c.done:
		return true
	default:
		return false
	}
}

func (c *guest) Exec(ctx context.Context, cmd contract.GuestCommand) (contract.GuestResult, error) {
	if c.closed() {
		return contract.GuestResult{}, ErrClosed
	}
	if cmd.Line == c.serving && c.serving != "" {
		return c.serve(ctx, cmd)
	}
	var in []byte
	if cmd.Stdin != nil {
		var err error
		if in, err = io.ReadAll(cmd.Stdin); err != nil {
			return contract.GuestResult{}, err
		}
	}
	c.vm.mu.Lock()
	a, ok := c.vm.answers[cmd.Line]
	if ok {
		c.vm.received[cmd.Line] = in
	}
	c.vm.mu.Unlock()
	if !ok && !c.fresh {
		a = Answer{Code: 127, Stderr: cmd.Line + ": not found\n"}
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
