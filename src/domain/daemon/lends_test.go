package daemon

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/jterrazz/studio-station/src/domain/machines"
)

type fakeLends struct {
	held     map[string][]Lend
	fail     error
	recalled []string
	during   func(vm string)
}

func (f *fakeLends) Held() (map[string][]Lend, error) { return f.held, nil }

func (f *fakeLends) Recall(_ context.Context, vm string, names []string) error {
	if f.during != nil {
		f.during(vm)
	}
	if f.fail != nil {
		return f.fail
	}
	for _, n := range names {
		f.recalled = append(f.recalled, vm+" "+n)
	}
	f.held[vm] = slices.DeleteFunc(f.held[vm], func(l Lend) bool { return slices.Contains(names, l.Name) })
	return nil
}

func lendHarness(t *testing.T, lends *fakeLends) (h harness, log *[]string) {
	t.Helper()
	h = newHarness(t,
		machines.VMRecord{Name: "api", Lifetime: machines.Lasting},
		machines.VMRecord{Name: "off", Lifetime: machines.Lasting},
	)
	h.d.o.Lends = lends
	lines := []string{}
	h.d.note = func(line string) { lines = append(lines, line) }
	for _, name := range []string{"api", "off"} {
		v, _ := h.d.find(name)
		v.state = StateStopped
		if name == "api" {
			v.state = StateRunning
		}
	}
	return h, &lines
}

func lend(name string, until time.Time) Lend {
	return Lend{Name: name, Until: until}
}

func TestALendPastItsUntilOnARunningVMIsRecalledAndLogged(t *testing.T) {
	now := time.Date(2026, 11, 1, 10, 0, 0, 0, time.UTC)
	lends := &fakeLends{held: map[string][]Lend{
		"api": {lend("operator:claude:oauth-token", now.Add(-time.Minute)), lend("session:github", now.Add(time.Hour))},
		"off": {lend("operator:claude:oauth-token", now.Add(-time.Minute))},
	}}
	h, log := lendHarness(t, lends)
	h.d.recallExpired(t.Context(), "")
	if !slices.Equal(lends.recalled, []string{"api operator:claude:oauth-token"}) {
		t.Errorf("recalled %v, want only api's expired lend (session:github is current, off is stopped)", lends.recalled)
	}
	want := "2026-11-01T10:00:00Z api lend: recalled operator:claude:oauth-token (expired)"
	if !slices.Contains(*log, want) {
		t.Errorf("daemon.log %q, want %q", *log, want)
	}
}

func TestTheRecordsAreSweptAtTheStartThenEveryMinute(t *testing.T) {
	h, _ := lendHarness(t, &fakeLends{held: map[string][]Lend{}})
	if !h.d.lendsDue() {
		t.Fatal("the first sweep was not due at the start")
	}
	h.clock.At = h.clock.At.Add(lendEvery - time.Second)
	if h.d.lendsDue() {
		t.Error("a sweep was due 59 s after the last one")
	}
	h.clock.At = h.clock.At.Add(time.Second)
	if !h.d.lendsDue() {
		t.Error("no sweep was due a minute after the last one")
	}
}

func TestAFailedRecallKeepsItsEntryAndIsLoggedOncePerHour(t *testing.T) {
	now := time.Date(2026, 11, 1, 10, 0, 0, 0, time.UTC)
	lends := &fakeLends{fail: errors.New("api: deleting the copies: exit 1"), held: map[string][]Lend{
		"api": {lend("operator:claude:oauth-token", now.Add(-time.Minute))},
	}}
	h, log := lendHarness(t, lends)
	failures := func() int {
		n := 0
		for _, l := range *log {
			if strings.Contains(l, "api lend: recall of operator:claude:oauth-token failed: api: deleting the copies: exit 1") {
				n++
			}
		}
		return n
	}
	for range 3 {
		h.d.recallExpired(t.Context(), "")
		h.clock.At = h.clock.At.Add(lendEvery)
	}
	if got := failures(); got != 1 {
		t.Errorf("%d failure lines over 3 minutes, want 1", got)
	}
	if len(lends.held["api"]) != 1 {
		t.Errorf("the failed lend's entry was dropped: %v", lends.held)
	}
	h.clock.At = now.Add(lendRetell)
	h.d.recallExpired(t.Context(), "")
	if got := failures(); got != 2 {
		t.Errorf("%d failure lines after an hour, want 2", got)
	}
}

func TestAStoppedVMsExpiredLendIsRecalledAtItsNextStartBeforeItsGuestIsConnected(t *testing.T) {
	now := time.Date(2026, 11, 1, 10, 0, 0, 0, time.UTC)
	lends := &fakeLends{held: map[string][]Lend{"off": {lend("operator:claude:oauth-token", now.Add(-time.Hour))}}}
	h, _ := lendHarness(t, lends)
	v, _ := h.d.find("off")
	v.state, v.guest = StateRunning, GuestConnecting
	var guestThen string
	lends.during = func(string) {
		h.d.mu.Lock()
		guestThen = v.guest
		h.d.mu.Unlock()
	}
	h.d.fromGuest(t.Context(), v, []byte(`{"v":1,"type":"ready"}`))
	if !slices.Equal(lends.recalled, []string{"off operator:claude:oauth-token"}) {
		t.Fatalf("recalled %v at the start, want off's expired lend", lends.recalled)
	}
	if guestThen == GuestConnected {
		t.Error("the guest was reported connected before the recall")
	}
	if v.guest != GuestConnected {
		t.Errorf("after the recall the guest is %s, want connected", v.guest)
	}
}
