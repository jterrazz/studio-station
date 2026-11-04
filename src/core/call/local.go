package call

import (
	"context"
	"reflect"

	"github.com/jterrazz/studio-station/src/core/fail"
	"github.com/jterrazz/studio-station/src/core/machine"
)

const Unknown fail.Code = "call.unknown"

type local interface {
	run(ctx context.Context, target machine.Profile, in any, p Progress) (any, error)
	stream(ctx context.Context, target machine.Profile, in any, p Progress) (<-chan Update[any], error)
}

func (r *Registry) Run(ctx context.Context, target machine.Profile, name string, in any, p Progress) (any, error) {
	// invariant: a call run in this process passes the same gate as a frame
	// (section, rule) and keeps its values as they are: nothing of In or Out
	// goes through the wire form, so an error inside an Out stays an error.
	h, err := r.local(name)
	if err != nil {
		return nil, err
	}
	return h.run(ctx, target, in, orSilent(p))
}

func (r *Registry) Stream(ctx context.Context, target machine.Profile, name string, in any, p Progress) (<-chan Update[any], error) {
	h, err := r.local(name)
	if err != nil {
		return nil, err
	}
	return h.stream(ctx, target, in, orSilent(p))
}

func (r *Registry) local(name string) (local, error) {
	h, ok := r.byName[name]
	if !ok {
		return nil, fail.New(fail.NotFound, Unknown, name, nil)
	}
	l, ok := h.(local)
	if !ok {
		return nil, fail.New(fail.NotFound, Unknown, name, nil)
	}
	return l, nil
}

func Set(out, v any) {
	// invariant: out is the pointer a Router was handed; a nil v leaves it as it is.
	if v == nil {
		return
	}
	reflect.ValueOf(out).Elem().Set(reflect.ValueOf(v))
}

func typed[In any](m Meta, target machine.Profile, in any) (In, error) {
	var zero In
	if err := gate(m, target); err != nil {
		return zero, err
	}
	switch v := in.(type) {
	case nil:
		return zero, nil
	case *In:
		return *v, nil
	case In:
		return v, nil
	}
	return zero, fail.New(fail.Refused, BadInput, m.Name, map[string]string{"in": m.In})
}

func (b bound[D, In, Out]) run(ctx context.Context, target machine.Profile, in any, p Progress) (any, error) {
	v, err := typed[In](b.meta, target, in)
	if err != nil {
		return nil, err
	}
	out, err := b.runFn(ctx, b.deps, v, p)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (b bound[D, In, Out]) stream(ctx context.Context, target machine.Profile, in any, p Progress) (<-chan Update[any], error) {
	out, err := b.run(ctx, target, in, p)
	if err != nil {
		return nil, err
	}
	once := make(chan Update[any], 1)
	once <- Update[any]{Value: out}
	close(once)
	return once, nil
}

func (w watched[D, In, Out]) stream(ctx context.Context, target machine.Profile, in any, p Progress) (<-chan Update[any], error) {
	v, err := typed[In](w.meta, target, in)
	if err != nil {
		return nil, err
	}
	updates, err := w.runFn(ctx, w.deps, v, p)
	if err != nil {
		return nil, err
	}
	out := make(chan Update[any])
	go func() {
		defer close(out)
		for u := range updates {
			select {
			case out <- Update[any]{Value: u.Value, Err: u.Err}:
			case <-ctx.Done():
				return
			}
		}
	}()
	return out, nil
}

func (w watched[D, In, Out]) run(ctx context.Context, target machine.Profile, in any, p Progress) (any, error) {
	updates, err := w.stream(ctx, target, in, p)
	if err != nil {
		return nil, err
	}
	var last any
	for u := range updates {
		if u.Err != nil {
			return nil, u.Err
		}
		last = u.Value
	}
	return last, ctx.Err()
}
