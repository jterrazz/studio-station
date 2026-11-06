package calls

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jterrazz/studio-station/src/activity"
	"github.com/jterrazz/studio-station/src/core/machine"
)

type processes map[int]activity.Usage

func (processes) Sample(context.Context) (activity.Sample, error) { return activity.Sample{}, nil }
func (processes) Top(context.Context) (activity.Busiest, error)   { return activity.Busiest{}, nil }
func (p processes) Process(_ context.Context, pid int) (activity.Usage, error) {
	if u, ok := p[pid]; ok {
		return u, nil
	}
	return activity.Usage{}, errors.New("no such process")
}

func TestAVMsRunIsMeasuredOnItsMacByTheProcessThatHoldsItsGuest(t *testing.T) {
	now := time.Date(2026, 11, 6, 4, 0, 0, 0, time.UTC)
	d := Deps{
		Meter: processes{4100: {CPU: 0, Memory: 1 << 20}, 4242: {CPU: 18, Memory: 15 << 30}},
		Name:  "jterrazz-mac-mini", Kind: machine.Mac, Now: func() time.Time { return now },
		Holder: func(_ context.Context, vm string) (int, error) {
			if vm == "infra" {
				return 4242, nil
			}
			return 0, nil
		},
	}
	r := d.read(t.Context(), ReadIn{Of: "infra", Machine: machine.Ref{Name: "infra", On: "jterrazz-mac-mini"}, Kind: machine.Server, Run: true})
	if r.Source != activity.FromMacProcess || r.Last.Run == nil || r.Last.Run.Memory != 15<<30 {
		t.Errorf("infra's run read on its Mac is %+v; want the 15 GB of the process that holds its guest", r)
	}
	if r = d.read(t.Context(), ReadIn{Of: "stopped", Run: true}); r.Last.Run != nil {
		t.Errorf("a VM no process holds read %+v; want no run", r.Last.Run)
	}
}
