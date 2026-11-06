package contract

import (
	"context"

	"github.com/jterrazz/studio-station/src/ports"
)

// value: the error a Tailnet read answers on a machine without the app.
type Absent = ports.Absent

type Tailnet interface {
	// invariant: one read answers this node, its sharing and its peers, so a
	// page that shows all three asks the tailnet once.
	Self(ctx context.Context) (ports.Self, error)
	Ready(ctx context.Context) (ports.SharingState, error)
	// invariant: a share is carried from this Mac's loopback only, never from
	// a wildcard or the tailnet address: the tailnet carries the bytes, so no
	// firewall stands in the way.
	Share(ctx context.Context, s ports.Share) (url string, err error)
	Unshare(ctx context.Context, s ports.Share) error
	Shared(ctx context.Context) ([]ports.Share, error)
	// invariant: never blocks a share: the certificate of the host the last
	// Ready named is asked for, and its issuance may take its time.
	Warm(ctx context.Context) error
}
