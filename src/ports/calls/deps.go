package calls

import (
	"context"

	"github.com/jterrazz/studio-station/src/core/call"
	"github.com/jterrazz/studio-station/src/core/machine"
	"github.com/jterrazz/studio-station/src/ports"
)

type Deps struct {
	Services Services
	Tailnet  Node
	Kind     machine.Kind
}

type Node interface {
	// value: this node, its peers and what it shares, each share told
	// station's when the services policy owns it.
	Status(ctx context.Context) (ports.Status, error)
}

type Services interface {
	CheckList(hereOnly bool) error
	List(ctx context.Context, hereOnly bool) (ports.Fleet, error)
	CheckURL(text string) (int, error)
	URL(ctx context.Context, port int) (ports.ServiceView, error)
	ParseTarget(text string) (ports.Target, error)
	// value: the host the target's VM runs on when it is not this one.
	CheckRelease(ctx context.Context, t ports.Target) (host string, err error)
	Pinned(t ports.Target) bool
	Release(ctx context.Context, t ports.Target, host string, yes bool) (ports.Reservation, error)
}

func All(d Deps) []call.Handler {
	return []call.Handler{call.Bind(List, d), call.Bind(URL, d), call.Bind(Release, d), call.Bind(Tailnet, d)}
}
