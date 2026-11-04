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

func Allocate(p Policy, leases []Lease, vm string, port int, busy func(int) bool) (Lease, error) {
	// invariant: a lease is sticky on (vm, guest port): an existing one is returned
	// as it is; a new one takes the LOWEST port of the range no lease holds and the
	// host does not hold either.
	if l, i := Find(leases, vm, port); i >= 0 {
		return l, nil
	}
	held, mine := map[int]bool{}, 0
	for _, l := range leases {
		held[l.HostPort] = true
		if l.VM == vm {
			mine++
		}
	}
	if mine >= p.MaxPerVM {
		return Lease{}, &MaxPerVMError{VM: vm, Max: p.MaxPerVM}
	}
	for hp := p.Low; hp <= p.High; hp++ {
		if held[hp] || (busy != nil && busy(hp)) {
			continue
		}
		return Lease{VM: vm, GuestPort: port, HostPort: hp}, nil
	}
	return Lease{}, &RangeFullError{Low: p.Low, High: p.High}
}
