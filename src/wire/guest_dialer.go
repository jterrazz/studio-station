package wire

import (
	"context"
	"io"
	"net/netip"

	"github.com/jterrazz/studio-station/src/engine"
	"github.com/jterrazz/studio-station/src/machines/adapter/ssh"
	machinescalls "github.com/jterrazz/studio-station/src/machines/calls"
	"github.com/jterrazz/studio-station/src/machines/contract"
)

// invariant: the one way into a VM: ssh, or the world's guests in a spec
// build.
func (m machine) guestDialer() contract.GuestDialer {
	if m.world != nil {
		return m.world.guests()
	}
	return ssh.New(m.host.FS, m.host.Sockets)
}

type engineGuests struct {
	dialer contract.GuestDialer
	vms    *machinescalls.Service
}

func (g engineGuests) Dial(ctx context.Context, vm string, at netip.AddrPort) (engine.Guest, error) {
	c, err := g.dialer.Dial(ctx, contract.GuestTarget{VM: vm, Address: at, User: g.vms.VMUser(), Key: g.vms.KeyPath(), KnownHosts: g.vms.KnownHostsPath(vm)})
	if err != nil {
		return nil, err
	}
	return engineGuest{c}, nil
}

type engineGuest struct{ contract.Guest }

func (g engineGuest) Exec(ctx context.Context, line string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
	res, err := g.Guest.Exec(ctx, contract.GuestCommand{Line: line, Stdin: stdin, Stdout: stdout, Stderr: stderr})
	return res.Code, err
}
