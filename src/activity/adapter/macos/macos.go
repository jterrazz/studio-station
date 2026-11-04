package macos

import (
	"context"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jterrazz/studio-station/src/activity"
	"github.com/jterrazz/studio-station/src/activity/contract"
	"github.com/jterrazz/studio-station/src/platform/local"
)

const budget = 2 * time.Second

type Runner interface {
	Output(ctx context.Context, c local.Cmd) (local.Output, error)
}

type Clock interface{ Now() time.Time }

type Meter struct {
	run   Runner
	clock Clock
}

var _ contract.Meter = (*Meter)(nil)

func New(run Runner, clock Clock) *Meter { return &Meter{run: run, clock: clock} }

func (m *Meter) ask(ctx context.Context, name string, args ...string) (string, bool) {
	out, err := m.run.Output(ctx, local.Cmd{Name: name, Args: args, Timeout: budget})
	if err != nil || out.ExitCode != 0 {
		return "", false
	}
	return out.Stdout, true
}

func (m *Meter) Sample(ctx context.Context) (activity.Sample, error) {
	if err := ctx.Err(); err != nil {
		return activity.Sample{}, err
	}
	sample := activity.Sample{At: m.clock.Now().UTC()}
	var wg sync.WaitGroup
	run := func(fn func()) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			fn()
		}()
	}
	run(func() { sample.CPU = processes(m.ask(ctx, "ps", "-arcwwwxo", "pid,%cpu,comm")) })
	run(func() { sample.Memory = scaleKB(processes(m.ask(ctx, "ps", "-amcwwwxo", "pid,rss,comm"))) })
	run(func() { sample.NetRx, sample.NetTx = network(m.ask(ctx, "netstat", "-ib")) })
	run(func() { sample.GPU = gpuLoad(m.ask(ctx, "ioreg", "-r", "-d", "1", "-c", "IOAccelerator")) })
	wg.Wait()
	if sample.CPU == nil {
		sample.CPU = []activity.Process{}
	}
	if sample.Memory == nil {
		sample.Memory = []activity.Process{}
	}
	return sample, nil
}

func (m *Meter) Uptime(ctx context.Context) (time.Duration, bool) {
	text, _ := m.ask(ctx, "sysctl", "-n", "kern.boottime")
	_, after, ok := strings.Cut(text, "sec = ")
	if !ok {
		return 0, false
	}
	boot, _, _ := strings.Cut(after, ",")
	sec := parseInt(boot)
	if sec <= 0 {
		return 0, false
	}
	return m.clock.Now().Sub(time.Unix(sec, 0)), true
}

func (m *Meter) Battery(ctx context.Context) string {
	// outside: pmset prints "-InternalBattery-0 (id=...)<TAB>92%; charging; 1:23 remaining";
	// a desktop prints no such line, and then there is no battery to report.
	text, ok := m.ask(ctx, "pmset", "-g", "batt")
	if !ok {
		return ""
	}
	for _, line := range strings.Split(text, "\n") {
		if !strings.Contains(line, "%") {
			continue
		}
		_, info, ok := strings.Cut(line, "\t")
		if !ok {
			continue
		}
		parts := strings.SplitN(strings.TrimSpace(info), ";", 3)
		if len(parts) >= 2 {
			return strings.TrimSpace(parts[0]) + " " + strings.TrimSpace(parts[1])
		}
	}
	return ""
}

func (m *Meter) Memory(ctx context.Context) int64 {
	text, _ := m.ask(ctx, "sysctl", "-n", "hw.memsize")
	return parseInt(text)
}

// outside: macOS ps prints a header, then "pid value command" with a command that
// may hold spaces ("Google Chrome Helper"), already sorted by the flag asked.
func processes(text string, ok bool) []activity.Process {
	var out []activity.Process
	if !ok {
		return out
	}
	lines := strings.Split(strings.TrimSpace(text), "\n")
	for i := 1; i < len(lines) && len(out) < activity.Top; i++ {
		fields := strings.Fields(lines[i])
		if len(fields) < 3 {
			continue
		}
		value, err := parseDecimal(fields[1])
		if err != nil {
			continue
		}
		out = append(out, activity.Process{PID: int(parseInt(fields[0])), Name: strings.Join(fields[2:], " "), Value: value})
	}
	return out
}

func scaleKB(ps []activity.Process) []activity.Process {
	for i := range ps {
		ps[i].Value *= 1024
	}
	return ps
}

// outside: ps prints its decimals in the caller's locale, `69,4` under fr_FR;
// it is not run under C, which would escape every non-ASCII command name.
func parseDecimal(s string) (float64, error) {
	return strconv.ParseFloat(strings.Replace(s, ",", ".", 1), 64)
}

// outside: the first en0 line carrying <Link#> holds the interface's raw counters.
func network(text string, _ bool) (rx, tx int64) {
	for _, line := range strings.Split(text, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 10 || !strings.HasPrefix(fields[0], "en0") || !strings.HasPrefix(fields[2], "<Link") {
			continue
		}
		return parseInt(fields[6]), parseInt(fields[9])
	}
	return 0, 0
}

func gpuLoad(text string, _ bool) float64 {
	_, after, ok := strings.Cut(text, `"Device Utilization %"=`)
	if !ok {
		return 0
	}
	end := strings.IndexFunc(after, func(c rune) bool { return (c < '0' || c > '9') && c != '.' })
	if end < 0 {
		end = len(after)
	}
	load, _ := strconv.ParseFloat(after[:end], 64)
	return load
}

func parseInt(s string) int64 {
	n, _ := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	return n
}
