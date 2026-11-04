package contracttest

import (
	"context"
	"testing"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/settings/contract"
)

type InspectorSubject struct {
	Inspector contract.Inspector
	// value: a protection this machine has on, and one it has off.
	On, Off catalog.Fact
	// value: a fact this machine cannot look at.
	Unreadable catalog.Fact
}

func Inspector(t *testing.T, subject func(t *testing.T) InspectorSubject) {
	ctx := context.Background()
	read := func(t *testing.T, i contract.Inspector, f catalog.Fact) (catalog.State, string) {
		t.Helper()
		got, err := i.Read(ctx, f)
		if err != nil {
			t.Fatalf("Read(%s): %v", f, err)
		}
		return got.State, got.Value
	}

	t.Run("a protection on reads on, one off reads off", func(t *testing.T) {
		s := subject(t)
		if state, value := read(t, s.Inspector, s.On); state != catalog.StateOK || value != "on" {
			t.Errorf("Read(%s) = %s %q; want ok on", s.On, state, value)
		}
		if state, value := read(t, s.Inspector, s.Off); state != catalog.StateAbsent || value != "off" {
			t.Errorf("Read(%s) = %s %q; want absent off", s.Off, state, value)
		}
	})

	t.Run("a fact the machine cannot look at is unknown, never off", func(t *testing.T) {
		s := subject(t)
		if state, value := read(t, s.Inspector, s.Unreadable); state != catalog.StateUnknown || value == "off" {
			t.Errorf("Read(%s) = %s %q; want unknown", s.Unreadable, state, value)
		}
	})
}
