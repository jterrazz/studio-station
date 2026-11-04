package contract

import (
	"context"

	"github.com/jterrazz/studio-station/src/machines"
)

type HealthProbe interface {
	// invariant: Probe is a server's "Check passes" step, run from its Mac:
	// nil once the URL answered 2xx or 3xx, or the host:port accepted a
	// connection, within machines.CheckBudget; any other end is its error.
	Probe(ctx context.Context, t machines.CheckTarget) error
}
