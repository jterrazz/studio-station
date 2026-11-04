package contracttest

import (
	"context"
	"slices"
	"testing"

	"github.com/jterrazz/studio-station/src/activity"
	"github.com/jterrazz/studio-station/src/activity/contract"
)

type MeterSubject struct {
	Meter contract.Meter
	// value: a meter of the same machine whose tools all fail.
	Silent contract.Meter
}

func Meter(t *testing.T, subject func(t *testing.T) MeterSubject) {
	ctx := context.Background()
	busiestFirst := func(ps []activity.Process) bool {
		return len(ps) <= activity.Top && slices.IsSortedFunc(ps, func(a, b activity.Process) int {
			switch {
			case a.Value > b.Value:
				return -1
			case a.Value < b.Value:
				return 1
			}
			return 0
		})
	}

	t.Run("a sample lists the busiest processes first, at most the top", func(t *testing.T) {
		s := subject(t)
		got, err := s.Meter.Sample(ctx)
		if err != nil {
			t.Fatalf("Sample: %v", err)
		}
		if len(got.CPU) == 0 || len(got.Memory) == 0 {
			t.Fatalf("Sample() = %+v; want processes by CPU and by memory", got)
		}
		if !busiestFirst(got.CPU) || !busiestFirst(got.Memory) {
			t.Errorf("Sample() = %+v; want at most %d each, busiest first", got, activity.Top)
		}
	})

	t.Run("a machine whose tools fail reads nothing, never an error", func(t *testing.T) {
		s := subject(t)
		got, err := s.Silent.Sample(ctx)
		if err != nil || got.CPU == nil || got.Memory == nil || len(got.CPU)+len(got.Memory) != 0 {
			t.Errorf("Sample() = %+v, %v; want empty lists and no error", got, err)
		}
		if _, ok := s.Silent.Uptime(ctx); ok {
			t.Error("Uptime() answered on a machine that did not say when it booted")
		}
		if b, m := s.Silent.Battery(ctx), s.Silent.Memory(ctx); b != "" || m != 0 {
			t.Errorf("Battery, Memory = %q, %d; want nothing read", b, m)
		}
	})

	t.Run("a sample past its context is an error", func(t *testing.T) {
		s := subject(t)
		done, cancel := context.WithCancel(ctx)
		cancel()
		if _, err := s.Meter.Sample(done); err == nil {
			t.Error("Sample() on an ended context answered")
		}
	})
}
