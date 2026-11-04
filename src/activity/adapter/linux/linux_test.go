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
		"cat /proc/uptime":         "3600.50 7000.00\n",
		"cat /proc/meminfo":        "MemTotal:        4028096 kB\nMemFree:  1000 kB\n",
	}
	contracttest.Meter(t, func(*testing.T) contracttest.MeterSubject {
		return contracttest.MeterSubject{Meter: New(vm, clock{}), Silent: New(answers{}, clock{})}
	})
	if got := New(vm, clock{}).Memory(context.Background()); got != 4028096*1024 {
		t.Errorf("Memory() = %d; want MemTotal in bytes", got)
	}
}
