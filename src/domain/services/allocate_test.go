package services

import (
	"errors"
	"testing"
)

func TestAllocateIsStickyAndTakesTheLowestFreePort(t *testing.T) {
	p := Policy{Low: 40000, High: 40002, MaxPerVM: 2}
	held := []Lease{{VM: "api", GuestPort: 8081, HostPort: 40000}}
	if l, err := Allocate(p, held, "api", 8081, nil); err != nil || l.HostPort != 40000 {
		t.Fatalf("a held lease moved: %+v %v", l, err)
	}
	busy := func(port int) bool { return port == 40001 }
	if l, err := Allocate(p, held, "web", 3000, busy); err != nil || l.HostPort != 40002 {
		t.Fatalf("lease %+v %v, want 40002 past the busy port", l, err)
	}
	var perVM *MaxPerVMError
	if _, err := Allocate(Policy{Low: 40000, High: 40009, MaxPerVM: 1}, held, "api", 3000, nil); !errors.As(err, &perVM) || err.Error() != "1 per VM" {
		t.Fatalf("err %v, want perVM per VM", err)
	}
	var full *RangeFullError
	if _, err := Allocate(Policy{Low: 40000, High: 40000, MaxPerVM: 9}, held, "web", 3000, nil); !errors.As(err, &full) || err.Error() != "range 40000-40000 full" {
		t.Fatalf("err %v, want range full", err)
	}
}
