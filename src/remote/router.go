package remote

import (
	"context"

	"github.com/jterrazz/studio-station/src/core/call"
	"github.com/jterrazz/studio-station/src/core/fail"
	"github.com/jterrazz/studio-station/src/core/machine"
)

type Router struct {
	registry *call.Registry
	here     machine.Profile
}

func NewRouter(registry *call.Registry, here machine.Profile) *Router {
	return &Router{registry: registry, here: here}
}

func (r *Router) Invoke(ctx context.Context, target machine.Ref, name string, in, out any, p call.Progress) error {
	if err := r.reach(target, name); err != nil {
		return err
	}
	v, err := r.registry.Run(ctx, r.here, name, in, p)
	call.Set(out, v)
	return err
}

func (r *Router) Watch(ctx context.Context, target machine.Ref, name string, in any, p call.Progress) (<-chan call.Update[any], error) {
	if err := r.reach(target, name); err != nil {
		return nil, err
	}
	return r.registry.Stream(ctx, r.here, name, in, p)
}

// invariant: the first two rows of the Router: this machine's calls run in
// this process, whatever their Where; a far machine is not reached by a call
// yet (--on still delegates the whole command).
func (r *Router) reach(target machine.Ref, name string) error {
	if target == (machine.Ref{}) || target == r.here.Ref {
		return nil
	}
	return fail.New(fail.Unreachable, call.Elsewhere, target.Name, map[string]string{"call": name})
}
