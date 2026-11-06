package engine

import (
	"bytes"
	"context"
	"io"
	"net/netip"
	"strings"
	"time"

	"github.com/jterrazz/studio-station/src/platform/local"
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

const VMStopped = "vm_stopped"

type GuestRan struct {
	Code   int    `json:"code"`
	Stdout string `json:"stdout"`
	Stderr string `json:"stderr"`
}

func (d *Daemon) GuestExec(ctx context.Context, vm, line, stdin string) (local.Output, error) {
	// invariant: a command inside a VM always rides the engine's own connection
	// to it; with none up, it is refused as absent, never run another way.
	v, err := d.find(vm)
	if err != nil {
		return local.Output{}, err
	}
	client, state := d.dialed(ctx, v)
	if client == nil && state == StateStopped {
		return local.Output{}, fault(VMStopped, vm+" is stopped", "station machines start "+vm)
	}
	if client == nil {
		return local.Output{}, fault("guest_absent", vm+"'s guest is not connected", "station machines start "+vm)
	}
	var out, errs bytes.Buffer
	code, err := client.Exec(ctx, line, strings.NewReader(stdin), &out, &errs)
	if err != nil {
		return local.Output{}, err
	}
	return local.Output{Stdout: out.String(), Stderr: errs.String(), ExitCode: code}, nil
}

const guestWait = 10 * time.Second

func (d *Daemon) dialed(ctx context.Context, v *vmState) (client Guest, state string) {
	// invariant: a call that lands while the guest is being dialed waits for
	// that dial, within guestWait, rather than being refused as absent.
	d.mu.Lock()
	client, state, up := v.client, v.state, v.up
	d.mu.Unlock()
	if client != nil || up == nil {
		return client, state
	}
	wait, cancel := context.WithTimeout(ctx, guestWait)
	defer cancel()
	select {
	case <-up:
	case <-wait.Done():
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	return v.client, v.state
}
