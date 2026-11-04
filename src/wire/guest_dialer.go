package wire

import (
	"context"
	"io"
	"net/netip"

	"github.com/jterrazz/studio-station/src/domain/machines"
	"github.com/jterrazz/studio-station/src/engine"
	"github.com/jterrazz/studio-station/src/machines/contract"
)

type engineGuests struct {
	dialer contract.GuestDialer
	vms    *machines.Service
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
