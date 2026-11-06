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
	// value: what the answering check says it answered; empty: not judged.
	Answer string
}

func CheckRunner(t *testing.T, subject func(t *testing.T) CheckRunnerSubject) {
	t.Run("a check that answers passes", func(t *testing.T) {
		s := subject(t)
		answer, err := s.Runner.Run(t.Context(), s.Answering())
		if err != nil {
			t.Fatalf("a check that answers failed: %v", err)
		}
		if s.Answer != "" && answer != s.Answer {
			t.Fatalf("a check that answers said %q, want %q", answer, s.Answer)
		}
	})
	t.Run("a check that no longer answers fails", func(t *testing.T) {
		s := subject(t)
		if _, err := s.Runner.Run(t.Context(), s.Refusing()); err == nil {
			t.Fatal("a check that does not answer passed")
		}
	})
}
