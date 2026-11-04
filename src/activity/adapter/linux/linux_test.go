package linux

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/jterrazz/studio-station/src/activity/contract/contracttest"
	"github.com/jterrazz/studio-station/src/platform/local"
)

type answers map[string]string

func (a answers) Output(_ context.Context, c local.Cmd) (local.Output, error) {
	out, ok := a[c.Name+" "+strings.Join(c.Args, " ")]
	if !ok {
		return local.Output{}, local.ErrNotFound
	}
	return local.Output{Stdout: out}, nil
}

type clock struct{}

func (clock) Now() time.Time { return time.Unix(1_800_000_000, 0) }

func TestMeter(t *testing.T) {
	vm := answers{
		"ps -eo pid,comm,%cpu,rss": "  PID COMMAND %CPU RSS\n    1 systemd 0.1 9000\n  42 node 12.5 204800\n  43 postgres 3.0 409600\n",
		"cat /proc/stat":           "cpu  100 0 50 800 50 0 0 0 0 0\ncpu0 50 0 25 400 25 0 0 0 0 0\ncpu1 50 0 25 400 25 0 0 0 0 0\n",
		"cat /proc/loadavg":        "1.40 1.50 1.60 2/300 4242\n",
		"cat /proc/uptime":         "3600.50 7000.00\n",
		"cat /proc/meminfo":        "MemTotal:        4028096 kB\nMemFree:  1000 kB\nMemAvailable: 2014048 kB\nSwapTotal: 0 kB\nSwapFree: 0 kB\n",
		"cat /proc/net/dev":        "Inter-|   Receive\n face |bytes\n    lo: 500 5 0 0 0 0 0 0 500 5 0 0 0 0 0 0\n  eth0: 9000 90 0 0 0 0 0 0 4000 40 0 0 0 0 0 0\n",
	}
	contracttest.Meter(t, func(*testing.T) contracttest.MeterSubject {
		return contracttest.MeterSubject{
			Meter: New(vm, clock{}), Want: contracttest.Totals{NetIn: 9000, NetOut: 4000}, Running: 42, Silent: New(answers{}, clock{}),
		}
	})
	s, _ := New(vm, clock{}).Sample(context.Background())
	if s.MemUsed != 2014048*1024 || s.CPUBusy != 150 || s.CPUTotal != 1000 || s.CPUs != 2 {
		t.Errorf("Sample() = %+v; want total minus available, the busy and all ticks, 2 CPUs", s)
	}
}
