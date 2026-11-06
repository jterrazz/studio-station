package contracttest

import (
	"testing"

	"github.com/jterrazz/studio-station/src/machines"
	"github.com/jterrazz/studio-station/src/machines/contract"
)

type CheckRunnerSubject struct {
	Runner contract.CheckRunner
	// value: a check that answers, then the same check once it stops
	// answering, each as the runner is handed it.
	Answering func() machines.CheckTarget
	Refusing  func() machines.CheckTarget
}

func CheckRunner(t *testing.T, subject func(t *testing.T) CheckRunnerSubject) {
	t.Run("a health check that answers passes", func(t *testing.T) {
		s := subject(t)
		if err := s.Runner.Run(t.Context(), s.Answering()); err != nil {
			t.Fatalf("a health check that answers failed: %v", err)
		}
	})
	t.Run("a health check that no longer answers fails", func(t *testing.T) {
		s := subject(t)
		if err := s.Runner.Run(t.Context(), s.Refusing()); err == nil {
			t.Fatal("a health check that does not answer passed")
		}
	})
}
