package engine

import (
	"context"
	"io"
	"net/netip"
)

type Guests interface {
	// invariant: what the engine needs of its one connection per VM: a command
	// inside the VM, a port forwarded to it, and the connection's end.
	Dial(ctx context.Context, vm string, at netip.AddrPort) (Guest, error)
}

type Guest interface {
	Exec(ctx context.Context, line string, stdin io.Reader, stdout, stderr io.Writer) (code int, err error)
	Forward(ctx context.Context, ip string, port int) (io.ReadWriteCloser, error)
	Close() error
	Done() <-chan struct{}
}
