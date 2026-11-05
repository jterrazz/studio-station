package contract

import (
	"context"
	"slices"

	"github.com/jterrazz/studio-station/src/ports"
)

type PublicRoute interface {
	// invariant: the word --via names the route by.
	Name() string
	// invariant: seen only when the route's own evidence is there on the
	// machine the port is declared on; nothing seen is an answer, never an
	// error.
	Verify(ctx context.Context, d ports.Declaration, on ports.Machine) (ports.Verified, error)
}

type PublicRoutes []PublicRoute

func (rs PublicRoutes) Named(name string) (PublicRoute, bool) {
	i := slices.IndexFunc(rs, func(r PublicRoute) bool { return r.Name() == name })
	if i < 0 {
		return nil, false
	}
	return rs[i], true
}

func (rs PublicRoutes) Names() []string {
	out := make([]string, len(rs))
	for i, r := range rs {
		out[i] = r.Name()
	}
	return out
}
