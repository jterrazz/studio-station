package remote

import (
	"context"
	"encoding/json"
	"slices"
	"sync"

	"github.com/jterrazz/studio-station/src/core/call"
	"github.com/jterrazz/studio-station/src/core/fail"
	"github.com/jterrazz/studio-station/src/core/machine"
	"github.com/jterrazz/studio-station/src/remote/contract"
	"github.com/jterrazz/studio-station/src/remote/serve"
)

type Reach struct {
	// value: another Mac, reached through its engine (sshstdio).
	Macs contract.EndpointDialer
	// value: this Mac's engine, which relays a call to one of its VMs (unix).
	Engine contract.EndpointDialer
	// value: where a bare name runs, from this Mac's own records: its On set
	// when it is a VM of this Mac; known false when no record names it.
	Locate func(name string) (ref machine.Ref, known bool)
	// value: the other Mac that holds a VM this Mac has no record of.
	Holder func(ctx context.Context, vm string) (mac string, err error)
	// value: the version Hello answers with.
	Version string
}

type Router struct {
	registry *call.Registry
	here     machine.Profile
	reach    Reach

	mu   sync.Mutex
	open map[string]*dialing
}

type dialing struct {
	once sync.Once
	ep   contract.Endpoint
	err  error
}

func NewRouter(registry *call.Registry, here machine.Profile, reach Reach) *Router {
	return &Router{registry: registry, here: here, reach: reach, open: map[string]*dialing{}}
}

func (r *Router) Invoke(ctx context.Context, target machine.Ref, name string, in, out any, p call.Progress) error {
	t, err := r.resolve(ctx, target)
	if err != nil {
		return err
	}
	if r.local(t) {
		v, err := r.registry.Run(ctx, r.here, name, in, p)
		call.Set(out, v)
		return err
	}
	ep, err := r.endpoint(ctx, t, name)
	if err != nil {
		return err
	}
	raw, err := json.Marshal(in)
	if err != nil {
		return err
	}
	answer, err := ep.Invoke(ctx, t, name, raw, p)
	if err != nil {
		return err
	}
	if decoded := call.Decode(answer, out); decoded != nil {
		e := fail.New(fail.Failed, call.BadOutput, name, map[string]string{"out": name})
		e.Cause = decoded
		return e
	}
	return nil
}

func (r *Router) Watch(ctx context.Context, target machine.Ref, name string, in any, p call.Progress) (<-chan call.Update[any], error) {
	t, err := r.resolve(ctx, target)
	if err != nil {
		return nil, err
	}
	if r.local(t) {
		return r.registry.Stream(ctx, r.here, name, in, p)
	}
	ep, err := r.endpoint(ctx, t, name)
	if err != nil {
		return nil, err
	}
	raw, err := json.Marshal(in)
	if err != nil {
		return nil, err
	}
	updates, err := ep.Watch(ctx, t, name, raw, p)
	if err != nil {
		return nil, err
	}
	return wire(ctx, updates), nil
}

func wire(ctx context.Context, updates <-chan call.Update[json.RawMessage]) <-chan call.Update[any] {
	out := make(chan call.Update[any])
	go func() {
		defer close(out)
		for u := range updates {
			next := call.Update[any]{Err: u.Err}
			if u.Err == nil {
				next.Value = u.Value
			}
			select {
			case out <- next:
			case <-ctx.Done():
				return
			}
		}
	}()
	return out
}

// invariant: the Router's rows, in order: no target or this machine runs in
// this process; a VM of this Mac rides this Mac's engine; any other machine
// rides the engine of the Mac it is on, which resolves it the same way; a
// name no record here holds is asked of the other Macs, never dialled.
func (r *Router) resolve(ctx context.Context, target machine.Ref) (machine.Ref, error) {
	if target == (machine.Ref{}) || target == r.here.Ref || target.Name == r.here.Ref.Name {
		return machine.Ref{}, nil
	}
	if target.On != "" || r.reach.Locate == nil {
		return target, nil
	}
	t, known := r.reach.Locate(target.Name)
	if known || r.reach.Holder == nil {
		return t, nil
	}
	mac, err := r.reach.Holder(ctx, target.Name)
	if err != nil {
		return machine.Ref{}, err
	}
	return machine.Ref{Name: target.Name, On: mac}, nil
}

func (r *Router) local(t machine.Ref) bool { return t == (machine.Ref{}) }

func (r *Router) endpoint(ctx context.Context, t machine.Ref, name string) (contract.Endpoint, error) {
	ep, hello, err := r.connect(ctx, t)
	if err != nil {
		return nil, err
	}
	if !slices.Contains(hello.Calls, name) {
		return nil, fail.New(fail.FarTooOld, call.FarTooOld, t.Mac(), map[string]string{"version": hello.Version, "call": name})
	}
	return ep, nil
}

func (r *Router) connect(ctx context.Context, t machine.Ref) (contract.Endpoint, contract.Hello, error) {
	dialer, mac := r.reach.Macs, t.Mac()
	if t.On != "" && t.On == r.here.Ref.Name {
		dialer = r.reach.Engine
	}
	if dialer == nil {
		return nil, contract.Hello{}, fail.New(fail.Unreachable, call.Unreachable, mac, nil)
	}
	r.mu.Lock()
	d, ok := r.open[mac]
	if !ok {
		d = &dialing{}
		r.open[mac] = d
	}
	r.mu.Unlock()
	d.once.Do(func() { d.ep, d.err = dialer.Dial(ctx, mac) })
	if d.err != nil {
		return nil, contract.Hello{}, d.err
	}
	hello, err := d.ep.Hello(ctx)
	return d.ep, hello, err
}

func (r *Router) Hello(ctx context.Context, mac string) (contract.Hello, error) {
	// value: what the far binary said at this connection: its version and its calls.
	_, hello, err := r.connect(ctx, machine.Ref{Name: mac})
	return hello, err
}

func (r *Router) Close() {
	r.mu.Lock()
	open := r.open
	r.open = map[string]*dialing{}
	r.mu.Unlock()
	for _, d := range open {
		if d.ep != nil {
			_ = d.ep.Close()
		}
	}
}

func (r *Router) Served() contract.Hello {
	return serve.Local(r.registry, r.here, r.reach.Version).Hello()
}

func (r *Router) Serve(ctx context.Context, target machine.Ref, name string, in json.RawMessage, p call.Progress) (any, error) {
	// invariant: a server answers for its own machine and for its VMs alone: a
	// frame for this machine runs here, one for one of its VMs rides its
	// engine's connection to that VM, and nothing hops to a third machine.
	vm, ok := r.vm(target)
	if !ok {
		return serve.Local(r.registry, r.here, r.reach.Version).Invoke(ctx, target, name, in, p)
	}
	ep, err := r.endpoint(ctx, vm, name)
	if err != nil {
		return nil, err
	}
	return ep.Invoke(ctx, vm, name, in, p)
}

func (r *Router) ServeWatch(ctx context.Context, target machine.Ref, name string, in json.RawMessage, p call.Progress) (<-chan call.Update[any], error) {
	vm, ok := r.vm(target)
	if !ok {
		return serve.Local(r.registry, r.here, r.reach.Version).Watch(ctx, target, name, in, p)
	}
	ep, err := r.endpoint(ctx, vm, name)
	if err != nil {
		return nil, err
	}
	updates, err := ep.Watch(ctx, vm, name, in, p)
	if err != nil {
		return nil, err
	}
	return wire(ctx, updates), nil
}

func (r *Router) vm(target machine.Ref) (machine.Ref, bool) {
	if target.Name == "" || target.Name == r.here.Ref.Name {
		return machine.Ref{}, false
	}
	if r.reach.Locate == nil {
		return machine.Ref{}, false
	}
	if t, _ := r.reach.Locate(target.Name); t.On != r.here.Ref.Name {
		return machine.Ref{}, false
	}
	return machine.Ref{Name: target.Name, On: r.here.Ref.Name}, true
}
