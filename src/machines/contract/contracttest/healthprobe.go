package contracttest

import (
	"testing"

	"github.com/jterrazz/studio-station/src/machines"
	"github.com/jterrazz/studio-station/src/machines/contract"
)

type HealthProbeSubject struct {
	Probe contract.HealthProbe
	// value: a check that answers, then the same check once it stops
	// answering, each as the probe is handed it.
	Answering func() machines.CheckTarget
	Refusing  func() machines.CheckTarget
}

func HealthProbe(t *testing.T, subject func(t *testing.T) HealthProbeSubject) {
	t.Run("a check that answers passes", func(t *testing.T) {
		s := subject(t)
		if err := s.Probe.Probe(t.Context(), s.Answering()); err != nil {
			t.Fatalf("a check that answers failed: %v", err)
		}
	})
	t.Run("a check that no longer answers fails", func(t *testing.T) {
		s := subject(t)
		if err := s.Probe.Probe(t.Context(), s.Refusing()); err == nil {
			t.Fatal("a check that does not answer passed")
		}
	})
}
