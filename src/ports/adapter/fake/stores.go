package fake

import (
	"context"
	"slices"
	"strings"

	"github.com/jterrazz/studio-station/src/core/store"
	"github.com/jterrazz/studio-station/src/ports"
	"github.com/jterrazz/studio-station/src/ports/contract"
)

type Reservations struct{ all []ports.Reservation }

var (
	_ contract.ReservationStore = (*Reservations)(nil)
	_ contract.DeclarationStore = (*Declarations)(nil)
)

func NewReservations() *Reservations { return &Reservations{} }

func (r *Reservations) Get(_ context.Context, vm string, guestPort int) (ports.Reservation, error) {
	for _, x := range r.all {
		if x.VM == vm && x.GuestPort == guestPort {
			return x, nil
		}
	}
	return ports.Reservation{}, store.ErrNotFound
}

func (r *Reservations) Put(ctx context.Context, res ports.Reservation) error {
	_ = r.Delete(ctx, res.VM, res.GuestPort)
	r.all = append(r.all, res)
	return nil
}

func (r *Reservations) List(context.Context) ([]ports.Reservation, error) {
	out := slices.Clone(r.all)
	slices.SortFunc(out, func(a, b ports.Reservation) int {
		if c := strings.Compare(a.VM, b.VM); c != 0 {
			return c
		}
		return a.GuestPort - b.GuestPort
	})
	return out, nil
}

func (r *Reservations) Delete(_ context.Context, vm string, guestPort int) error {
	r.all = slices.DeleteFunc(r.all, func(x ports.Reservation) bool { return x.VM == vm && x.GuestPort == guestPort })
	return nil
}

func (r *Reservations) Migrate(context.Context) error { return nil }

type Declarations struct{ all []ports.Declaration }

func NewDeclarations() *Declarations { return &Declarations{} }

func (d *Declarations) Get(_ context.Context, machine string, port int) (ports.Declaration, error) {
	for _, x := range d.all {
		if x.Machine == machine && x.Port == port {
			return x, nil
		}
	}
	return ports.Declaration{}, store.ErrNotFound
}

func (d *Declarations) Put(ctx context.Context, decl ports.Declaration) error {
	_ = d.Delete(ctx, decl.Machine, decl.Port)
	d.all = append(d.all, decl)
	return nil
}

func (d *Declarations) List(context.Context) ([]ports.Declaration, error) {
	out := slices.Clone(d.all)
	slices.SortFunc(out, func(a, b ports.Declaration) int {
		if c := strings.Compare(a.Machine, b.Machine); c != 0 {
			return c
		}
		return a.Port - b.Port
	})
	return out, nil
}

func (d *Declarations) Delete(_ context.Context, machine string, port int) error {
	d.all = slices.DeleteFunc(d.all, func(x ports.Declaration) bool { return x.Machine == machine && x.Port == port })
	return nil
}

func (d *Declarations) Migrate(context.Context) error { return nil }
