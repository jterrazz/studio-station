package dial

import (
	"context"

	"github.com/jterrazz/studio-station/src/machines"
	"github.com/jterrazz/studio-station/src/machines/contract"
	"github.com/jterrazz/studio-station/src/platform/local"
)

type Probe struct{}

var _ contract.HealthProbe = Probe{}

func (Probe) Probe(ctx context.Context, t machines.CheckTarget) error {
	return local.Answers(ctx, local.Check{Dial: t.Dial, URL: t.URL, Name: t.Name, Verify: t.Verify, Budget: machines.CheckBudget})
}
