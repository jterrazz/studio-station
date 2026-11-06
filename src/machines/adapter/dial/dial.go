package dial

import (
	"context"
	"strconv"

	"github.com/jterrazz/studio-station/src/machines"
	"github.com/jterrazz/studio-station/src/machines/contract"
	"github.com/jterrazz/studio-station/src/platform/local"
)

type Checker struct{}

var _ contract.CheckRunner = Checker{}

func (Checker) Run(ctx context.Context, t machines.CheckTarget) (string, error) {
	status, err := local.Answer(ctx, local.Check{Dial: t.Dial, URL: t.URL, Name: t.Name, Verify: t.Verify, Budget: machines.CheckBudget})
	if err != nil || status == 0 {
		return "", err
	}
	return strconv.Itoa(status), nil
}
