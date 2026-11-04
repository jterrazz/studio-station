package remote

import (
	"context"

	"github.com/jterrazz/studio-station/src/core/call"
	"github.com/jterrazz/studio-station/src/core/fail"
	"github.com/jterrazz/studio-station/src/core/machine"
	"github.com/jterrazz/studio-station/src/remote/api"
)

type Router struct {
	registry *call.Registry
	here     machine.Profile
	// value: this Mac's engine, where a call that needs its state runs; nil
	// runs every call of this machine in this process.
	engine call.Router
}

func NewRouter(registry *call.Registry, here machine.Profile) *Router {
	return &Router{registry: registry, here: here}
}

func (r *Router) WithEngine(engine call.Router) *Router {
	// value: the router with this Mac's engine as its third row.
	out := *r
	out.engine = engine
	return &out
}

func (r *Router) Invoke(ctx context.Context, target machine.Ref, name string, in, out any, p call.Progress) error {
	if err := r.reach(target, name); err != nil {
		return err
	}
	if r.inEngine(name) {
		return r.engine.Invoke(ctx, target, name, in, out, p)
	}
	v, err := r.registry.Run(ctx, r.here, name, in, p)
	call.Set(out, v)
	return err
}

func (r *Router) Watch(ctx context.Context, target machine.Ref, name string, in any, p call.Progress) (<-chan call.Update[any], error) {
	if err := r.reach(target, name); err != nil {
		return nil, err
	}
	if r.inEngine(name) {
		return r.engine.Watch(ctx, target, name, in, p)
	}
	return r.registry.Stream(ctx, r.here, name, in, p)
}

func (r *Router) inEngine(name string) bool {
	if r.engine == nil {
		return false
	}
	h, ok := r.registry.Lookup(name)
	return ok && h.Meta().Where == call.InEngine
}

// invariant: the rows of the Router this machine reaches: its calls run in
// this process, those that need the engine's state in this Mac's engine when
// it has one; a far machine is not reached by a call yet (--on still
// delegates the whole command).
func (r *Router) reach(target machine.Ref, name string) error {
	if target == (machine.Ref{}) || target == r.here.Ref {
		return nil
	}
	return fail.New(fail.Unreachable, call.Elsewhere, target.Name, map[string]string{"call": name})
}

func Endpoint(c api.Client) call.Router {
	// value: a station.api/v2 server reached as a Router: each call a frame,
	// each watch its own connection, its updates in their wire form.
	return endpoint{c}
}

type endpoint struct{ c api.Client }

func (e endpoint) Invoke(ctx context.Context, target machine.Ref, name string, in, out any, p call.Progress) error {
	return e.c.Call(ctx, name, target, in, out, p)
}

func (e endpoint) Watch(ctx context.Context, _ machine.Ref, name string, in any, _ call.Progress) (<-chan call.Update[any], error) {
	raw, err := e.c.Watch(ctx, name, in)
	if err != nil {
		return nil, err
	}
	out := make(chan call.Update[any])
	go func() {
		defer close(out)
		for v := range raw {
			select {
			case out <- call.Update[any]{Value: v}:
			case <-ctx.Done():
				return
			}
		}
	}()
	return out, nil
}
