package fake

import (
	"testing"

	"github.com/jterrazz/studio-station/src/machines"
	"github.com/jterrazz/studio-station/src/machines/contract"
	"github.com/jterrazz/studio-station/src/machines/contract/contracttest"
)

func TestTheFakeHypervisorKeepsTheContract(t *testing.T) {
	contracttest.Hypervisor(t, func(*testing.T) contracttest.Subject {
		h := New("vz")
		return contracttest.Subject{
			Hypervisor: h,
			End: func(r contract.Run, how machines.EndReason) {
				end := machines.RunEnd{Reason: how}
				if how == machines.Crashed {
					end.ExitCode = 1
				}
				h.End(r.VM(), end)
			},
			Restart: func() contract.Hypervisor { return h },
		}
	})
}
