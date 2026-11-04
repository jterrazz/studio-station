package fake

import (
	"testing"

	"github.com/jterrazz/studio-station/src/machines"
	"github.com/jterrazz/studio-station/src/machines/contract/contracttest"
	"github.com/jterrazz/studio-station/src/world"
)

func TestTheFakeChecksKeepTheHealthProbeContract(t *testing.T) {
	contracttest.HealthProbe(t, func(*testing.T) contracttest.HealthProbeSubject {
		w := &world.World{Here: "mac-mini"}
		w.Mac("mac-mini").VMs = map[string]*world.VM{"infra": {PID: 2001}}
		return contracttest.HealthProbeSubject{
			Probe:     HealthProbeOf(w),
			Answering: func() machines.CheckTarget { return machines.CheckTarget{Dial: "192.168.64.5:6443", Name: "infra"} },
			Refusing: func() machines.CheckTarget {
				w.Script = append(w.Script, world.Fault{Call: "HealthProbe.Probe"})
				return machines.CheckTarget{Dial: "192.168.64.5:6443", Name: "infra"}
			},
		}
	})
}
