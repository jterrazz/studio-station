package remote

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
	open map[string]*link
}

type link struct {
	// invariant: one connection to a Mac, shared by every call while it
	// lives: the callers that wait on one dial share its answer, a failed dial
	// is never kept, and a connection that broke is dialed again.
	done chan struct{}
	ep   contract.Endpoint
	err  error
}

func NewRouter(registry *call.Registry, here machine.Profile, reach Reach) *Router {
	return &Router{registry: registry, here: here, reach: reach, open: map[string]*link{}}
}

func (r *Router) Invoke(ctx context.Context, target machine.Ref, name string, in, out any, p call.Progress) error {
	t, err := r.resolve(ctx, target)
	if err != nil {
		return err
	}
	if r.local(t) {
		v, ran := r.registry.Run(ctx, r.here, name, in, p)
		call.Set(out, v)
		return ran
	}
	l, err := r.endpoint(ctx, t, name)
	if err != nil {
		return err
	}
	raw, err := json.Marshal(in)
	if err != nil {
		return err
	}
	answer, err := l.ep.Invoke(ctx, t, name, raw, p)
	if err != nil {
		r.broke(t.Mac(), l, err)
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
	l, err := r.endpoint(ctx, t, name)
	if err != nil {
		return nil, err
	}
	raw, err := json.Marshal(in)
	if err != nil {
		return nil, err
	}
	updates, err := l.ep.Watch(ctx, t, name, raw, p)
	if err != nil {
		r.broke(t.Mac(), l, err)
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
// name no record here holds is asked of the other Macs, never dialed.
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

func (r *Router) endpoint(ctx context.Context, t machine.Ref, name string) (*link, error) {
	// invariant: a call the far Hello lacks is asked again of a fresh
	// connection, so a Mac upgraded since its connection was opened serves it.
	l, hello, fresh, err := r.connect(ctx, t)
	if err == nil && !fresh && !slices.Contains(hello.Calls, name) {
		r.drop(t.Mac(), l)
		l, hello, _, err = r.connect(ctx, t)
	}
	if err != nil {
		return nil, err
	}
	if !slices.Contains(hello.Calls, name) {
		return nil, fail.New(fail.FarTooOld, call.FarTooOld, t.Mac(), map[string]string{"version": hello.Version, "call": name})
	}
	return l, nil
}

func (r *Router) connect(ctx context.Context, t machine.Ref) (l *link, hello contract.Hello, fresh bool, err error) {
	dialer, mac := r.reach.Macs, t.Mac()
	if t.On != "" && t.On == r.here.Ref.Name {
		dialer = r.reach.Engine
	}
	if dialer == nil {
		return nil, contract.Hello{}, false, fail.New(fail.Unreachable, call.Unreachable, mac, nil)
	}
	r.mu.Lock()
	l, kept := r.open[mac]
	if !kept {
		l = &link{done: make(chan struct{})}
		r.open[mac] = l
	}
	r.mu.Unlock()
	if !kept {
		l.ep, l.err = dialer.Dial(ctx, mac)
		if l.err != nil {
			r.forget(mac, l)
		}
		close(l.done)
	}
	select {
	case <-l.done:
	case <-ctx.Done():
		return nil, contract.Hello{}, false, ctx.Err()
	}
	if l.err != nil {
		return nil, contract.Hello{}, false, l.err
	}
	hello, err = l.ep.Hello(ctx)
	if err != nil {
		r.broke(mac, l, err)
		return nil, contract.Hello{}, false, err
	}
	return l, hello, !kept, nil
}

func (r *Router) broke(mac string, l *link, err error) {
	// invariant: only the connection's own end drops it: a far side that
	// answers that one of its VMs is unreachable keeps its connection.
	var e *fail.Error
	if errors.As(err, &e) && e.Type == fail.Unreachable && e.Subject == mac {
		r.drop(mac, l)
	}
}

func (r *Router) drop(mac string, l *link) {
	if r.forget(mac, l) {
		_ = l.ep.Close()
	}
}

func (r *Router) forget(mac string, l *link) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.open[mac] != l {
		return false
	}
	delete(r.open, mac)
	return true
}

func (r *Router) Hello(ctx context.Context, mac string) (contract.Hello, error) {
	// value: what the far binary says on the connection open now: its version and its calls.
	_, hello, _, err := r.connect(ctx, machine.Ref{Name: mac})
	return hello, err
}

func (r *Router) Close() {
	r.mu.Lock()
	open := r.open
	r.open = map[string]*link{}
	r.mu.Unlock()
	for _, l := range open {
		select {
		case <-l.done:
			if l.ep != nil {
				_ = l.ep.Close()
			}
		default:
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
	l, err := r.endpoint(ctx, vm, name)
	if err != nil {
		return nil, err
	}
	out, err := l.ep.Invoke(ctx, vm, name, in, p)
	if err != nil {
		r.broke(vm.Mac(), l, err)
	}
	return out, err
}

func (r *Router) ServeWatch(ctx context.Context, target machine.Ref, name string, in json.RawMessage, p call.Progress) (<-chan call.Update[any], error) {
	vm, ok := r.vm(target)
	if !ok {
		return serve.Local(r.registry, r.here, r.reach.Version).Watch(ctx, target, name, in, p)
	}
	l, err := r.endpoint(ctx, vm, name)
	if err != nil {
		return nil, err
	}
	updates, err := l.ep.Watch(ctx, vm, name, in, p)
	if err != nil {
		r.broke(vm.Mac(), l, err)
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
