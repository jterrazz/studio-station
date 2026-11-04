package services

import (
	"context"
	"reflect"

	"github.com/jterrazz/studio-station/src/ports"
	portscontract "github.com/jterrazz/studio-station/src/ports/contract"
)

const Schema = "station.services/v1"

type Lease = ports.Reservation

type Reservations = portscontract.ReservationStore

func SaveLeases(ctx context.Context, r Reservations, leases []Lease) error {
	// invariant: saving the whole set changes only what moved: a lease gone is
	// deleted, a lease added or changed is put, each write whole.
	current, err := r.List(ctx)
	if err != nil {
		return err
	}
	want := make(map[string]Lease, len(leases))
	for _, l := range leases {
		want[l.Key()] = l
	}
	for _, c := range current {
		if _, ok := want[c.Key()]; !ok {
			if err := r.Delete(ctx, c.VM, c.GuestPort); err != nil {
				return err
			}
		}
	}
	had := make(map[string]Lease, len(current))
	for _, c := range current {
		had[c.Key()] = c
	}
	for _, l := range leases {
		if c, ok := had[l.Key()]; ok && reflect.DeepEqual(c, l) {
			continue
		}
		if err := r.Put(ctx, l); err != nil {
			return err
		}
	}
	return nil
}

func Find(leases []Lease, vm string, port int) (found Lease, index int) {
	for i, l := range leases {
		if l.VM == vm && l.GuestPort == port {
			return l, i
		}
	}
	return Lease{}, -1
}

func Without(leases []Lease, keep func(Lease) bool) (kept, gone []Lease) {
	for _, l := range leases {
		if keep(l) {
			kept = append(kept, l)
		} else {
			gone = append(gone, l)
		}
	}
	return kept, gone
}
