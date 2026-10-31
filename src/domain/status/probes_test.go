package status

import (
	"context"
	"testing"
	"time"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/system"
)

type slowRunner struct{ system.Runner }

func (slowRunner) Output(ctx context.Context, c system.Cmd) (system.Output, error) {
	if c.Name == "slow" {
		<-ctx.Done()
		return system.Output{}, system.ErrTimeout
	}
	return system.Output{Stdout: "192.168.1.2\n"}, nil
}

func TestAProbePastItsBudgetAnswersUnknownAndDelaysNoOtherRow(t *testing.T) {
	s := New(system.Host{Run: slowRunner{}}, Sources{}, Options{})
	table := []probe{
		{name: "stuck", cmd: cmd("slow"), budget: 30 * time.Millisecond, interpret: firstField},
		{name: "local ip", cmd: cmd("fast"), interpret: firstField},
	}

	got := s.probes(context.Background(), table)

	if got[0].State != catalog.StateUnknown {
		t.Errorf("a probe past its budget answered %+v, want state unknown", got[0])
	}
	if got[1].State != catalog.StateOK || got[1].Value != "192.168.1.2" {
		t.Errorf("the prompt probe answered %+v, want its value", got[1])
	}
}
