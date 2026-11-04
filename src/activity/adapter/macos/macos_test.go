package macos

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
	mac := answers{
		"ps -arcwwwxo pid,%cpu,comm": "  PID  %CPU COMMAND\n  301  45,5 Google Chrome Helper\n  302  12.0 node\n",
		"ps -amcwwwxo pid,rss,comm":  "  PID    RSS COMMAND\n  302 2097152 node\n  301  524288 Google Chrome Helper\n",
		"sysctl -n kern.boottime":    "{ sec = 1799996400, usec = 0 } Thu Jan 15 08:00:00 2027\n",
		"sysctl -n hw.memsize":       "17179869184\n",
		"pmset -g batt":              "Now drawing from 'Battery Power'\n -InternalBattery-0 (id=1)\t92%; discharging; 4:10 remaining present: true\n",
	}
	contracttest.Meter(t, func(*testing.T) contracttest.MeterSubject {
		return contracttest.MeterSubject{Meter: New(mac, clock{}), Silent: New(answers{}, clock{})}
	})
	m := New(mac, clock{})
	if up, ok := m.Uptime(context.Background()); !ok || up != time.Hour {
		t.Errorf("Uptime() = %v, %v; want 1h", up, ok)
	}
	if got := m.Battery(context.Background()); got != "92% discharging" {
		t.Errorf("Battery() = %q; want 92%% discharging", got)
	}
}
