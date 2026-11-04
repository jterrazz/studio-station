package engine

import (
	"bytes"
	"context"
	"io"
	"net/netip"
	"strings"

	"github.com/jterrazz/studio-station/src/system"
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

type GuestRan struct {
	Code   int    `json:"code"`
	Stdout string `json:"stdout"`
	Stderr string `json:"stderr"`
}

func (d *Daemon) GuestExec(ctx context.Context, vm, line, stdin string) (system.Output, error) {
	// invariant: a command inside a VM always rides the engine's own connection
	// to it; with none up, it is refused as absent, never run another way.
	v, err := d.find(vm)
	if err != nil {
		return system.Output{}, err
	}
	d.mu.Lock()
	client := v.client
	d.mu.Unlock()
	if client == nil {
		return system.Output{}, fault("guest_absent", vm+"'s guest is not connected", "station machines start "+vm)
	}
	var out, errs bytes.Buffer
	code, err := client.Exec(ctx, line, strings.NewReader(stdin), &out, &errs)
	if err != nil {
		return system.Output{}, err
	}
	return system.Output{Stdout: out.String(), Stderr: errs.String(), ExitCode: code}, nil
}
