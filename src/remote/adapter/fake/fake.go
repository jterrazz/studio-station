package fake

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"sync"
	"sync/atomic"

	"github.com/jterrazz/studio-station/src/core/call"
	"github.com/jterrazz/studio-station/src/core/fail"
	"github.com/jterrazz/studio-station/src/core/machine"
	"github.com/jterrazz/studio-station/src/remote/contract"
)

type Mac struct {
	Registry *call.Registry
	Profile  machine.Profile
	Version  string
}

type Dialer struct {
	// invariant: a second Mac in this process: its calls run on its own
	// registry, every value through the wire form, as over ssh. A connection
	// keeps the Mac it was dialed to, as a far process keeps its binary, and
	// breaks when that Mac goes down or when it is closed.
	mu     sync.Mutex
	macs   map[string]Mac
	down   map[string]bool
	dialed []string
}

func New() *Dialer { return &Dialer{macs: map[string]Mac{}, down: map[string]bool{}} }

func (d *Dialer) Serve(name string, m Mac) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.macs[name] = m
	delete(d.down, name)
}

func (d *Dialer) Down(name string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.down[name] = true
}

func (d *Dialer) Dial(_ context.Context, mac string) (contract.Endpoint, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.dialed = append(d.dialed, mac)
	m, ok := d.macs[mac]
	if !ok || d.down[mac] {
		return nil, fail.New(fail.Unreachable, call.Unreachable, mac, nil)
	}
	return endpoint{d: d, name: mac, m: m, closed: new(atomic.Bool)}, nil
}

func (d *Dialer) Dialed() []string {
	// value: every name Dial was asked for, in order.
	d.mu.Lock()
	defer d.mu.Unlock()
	return slices.Clone(d.dialed)
}

type endpoint struct {
	d      *Dialer
	name   string
	m      Mac
	closed *atomic.Bool
}

func (e endpoint) broken() error {
	e.d.mu.Lock()
	defer e.d.mu.Unlock()
	if e.d.down[e.name] || e.closed.Load() {
		return fail.New(fail.Unreachable, call.Unreachable, e.name, nil)
	}
	return nil
}

func (e endpoint) Hello(context.Context) (contract.Hello, error) {
	if err := e.broken(); err != nil {
		return contract.Hello{}, err
	}
	h := contract.Hello{Version: e.m.Version, Calls: []string{}}
	for _, meta := range e.m.Registry.Metas() {
		h.Calls = append(h.Calls, meta.Name)
	}
	return h, nil
}

func (e endpoint) Invoke(ctx context.Context, _ machine.Ref, name string, in json.RawMessage, p call.Progress) (json.RawMessage, error) {
	if err := e.broken(); err != nil {
		return nil, err
	}
	v, err := e.m.Registry.Input(name, in)
	if err != nil {
		return nil, over(err)
	}
	out, err := e.m.Registry.Run(ctx, e.m.Profile, name, v, p)
	if err != nil {
		return nil, over(err)
	}
	return call.Encode(out, worded)
}

func (e endpoint) Watch(ctx context.Context, _ machine.Ref, name string, in json.RawMessage, p call.Progress) (<-chan call.Update[json.RawMessage], error) {
	if err := e.broken(); err != nil {
		return nil, err
	}
	v, err := e.m.Registry.Input(name, in)
	if err != nil {
		return nil, over(err)
	}
	updates, err := e.m.Registry.Stream(ctx, e.m.Profile, name, v, p)
	if err != nil {
		return nil, over(err)
	}
	out := make(chan call.Update[json.RawMessage])
	go func() {
		defer close(out)
		for u := range updates {
			if err := e.broken(); err != nil {
				out <- call.Update[json.RawMessage]{Err: err}
				return
			}
			next := call.Update[json.RawMessage]{Err: over(u.Err)}
			if u.Err == nil {
				next.Value, next.Err = call.Encode(u.Value, worded)
			}
			out <- next
		}
	}()
	return out, nil
}

func (e endpoint) Close() error {
	e.closed.Store(true)
	return nil
}

func worded(err error) *fail.Error {
	var e *fail.Error
	if errors.As(err, &e) {
		return e
	}
	return &fail.Error{Message: err.Error(), Exit: fail.ExitFailure}
}

func over(err error) error {
	if err == nil {
		return nil
	}
	return call.ToWire(err, worded).Err()
}
