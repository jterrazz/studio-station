package fake

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"sync"

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
	// registry, every value through the wire form, as over ssh.
	mu      sync.Mutex
	macs    map[string]Mac
	down    map[string]bool
	dialled []string
}

func New() *Dialer { return &Dialer{macs: map[string]Mac{}, down: map[string]bool{}} }

func (d *Dialer) Serve(name string, m Mac) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.macs[name] = m
}

func (d *Dialer) Down(name string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.down[name] = true
}

func (d *Dialer) Dial(_ context.Context, mac string) (contract.Endpoint, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.dialled = append(d.dialled, mac)
	m, ok := d.macs[mac]
	if !ok || d.down[mac] {
		return nil, fail.New(fail.Unreachable, call.Unreachable, mac, nil)
	}
	return endpoint{m: m}, nil
}

func (d *Dialer) Dialled() []string {
	// value: every name Dial was asked for, in order.
	d.mu.Lock()
	defer d.mu.Unlock()
	return slices.Clone(d.dialled)
}

type endpoint struct{ m Mac }

func (e endpoint) Hello(context.Context) (contract.Hello, error) {
	h := contract.Hello{Version: e.m.Version, Calls: []string{}}
	for _, meta := range e.m.Registry.Metas() {
		h.Calls = append(h.Calls, meta.Name)
	}
	return h, nil
}

func (e endpoint) Invoke(ctx context.Context, _ machine.Ref, name string, in json.RawMessage, p call.Progress) (json.RawMessage, error) {
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
			next := call.Update[json.RawMessage]{Err: over(u.Err)}
			if u.Err == nil {
				next.Value, next.Err = call.Encode(u.Value, worded)
			}
			out <- next
		}
	}()
	return out, nil
}

func (endpoint) Close() error { return nil }

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
