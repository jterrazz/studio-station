package calls

import (
	"context"
	"errors"
	"strings"

	"github.com/jterrazz/studio-station/src/setup"

	"github.com/jterrazz/studio-station/src/catalog"
)

const stepSettings = "settings"

type Setting struct {
	Name string
	// value: Not applied, on a machine it lives on (its app installed).
	Due bool
	// value: applied, then changed by someone: only the owner may replace it.
	ChangedSince bool
	AlwaysOn     bool
	NeedsOwner   bool
}

type Settings interface {
	List(ctx context.Context) ([]Setting, error)
	Apply(ctx context.Context, name string) error
}

// invariant: setup applies every Not applied setting that needs nobody, lists
// the others due and every Changed since one as next steps, and never applies
// nor lists the Always-on Mac group (opt-in); a setting it could not read is
// left alone; one that fails leaves the others applied.
func (s *Provisioner) settings(ctx context.Context) ([]setup.Step, error) {
	if s.o.Settings == nil {
		return nil, nil
	}
	list, err := s.o.Settings.List(ctx)
	if err != nil {
		step, stepErr := failed(stepSettings, err)
		return []setup.Step{step}, stepErr
	}
	var applied, next, broken []string
	var errs []error
	for _, it := range list {
		switch {
		case it.AlwaysOn, !it.Due && !it.ChangedSince:
		case it.NeedsOwner, it.ChangedSince:
			next = append(next, it.Name)
		default:
			if err := s.o.Settings.Apply(ctx, it.Name); err != nil {
				broken = append(broken, it.Name+": "+err.Error())
				errs = append(errs, err)
				continue
			}
			applied = append(applied, it.Name)
		}
	}
	var done []string
	if len(applied) > 0 {
		done = append(done, "applied "+strings.Join(applied, " "))
	}
	step := setup.Step{Name: stepSettings, State: catalog.StateOK, Detail: strings.Join(append(done, broken...), ", "), Changed: len(applied) > 0}
	var stepErr error
	if len(broken) > 0 {
		step.State = catalog.StateBroken
		stepErr = &setup.StepFailedError{Step: stepSettings, Cause: errors.Join(errs...)}
	}
	steps := []setup.Step{step}
	for _, name := range next {
		steps = append(steps, setup.Step{Name: name, State: catalog.StateAbsent, Detail: "next: station settings apply " + name})
	}
	return steps, stepErr
}
