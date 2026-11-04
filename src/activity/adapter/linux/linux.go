package linux

import (
	"context"
	"sort"
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
	wg.Add(2)
	go func() {
		defer wg.Done()
		sample.CPU, sample.Memory = processes(m.ask(ctx, "ps", "-eo", "pid,comm,%cpu,rss"))
	}()
	go func() {
		defer wg.Done()
		sample.NetRx, sample.NetTx = network(m.ask(ctx, "cat", "/proc/net/dev"))
	}()
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
	text, ok := m.ask(ctx, "cat", "/proc/uptime")
	field, _, _ := strings.Cut(strings.TrimSpace(text), " ")
	seconds, err := strconv.ParseFloat(field, 64)
	if !ok || err != nil {
		return 0, false
	}
	return time.Duration(seconds * float64(time.Second)), true
}

func (m *Meter) Battery(context.Context) string { return "" }

func (m *Meter) Memory(ctx context.Context) int64 {
	text, _ := m.ask(ctx, "cat", "/proc/meminfo")
	for _, line := range strings.Split(text, "\n") {
		if rest, ok := strings.CutPrefix(line, "MemTotal:"); ok {
			if fields := strings.Fields(rest); len(fields) > 0 {
				return parseInt(fields[0]) * 1024
			}
		}
	}
	return 0
}

func processes(text string, ok bool) (cpu, memory []activity.Process) {
	if !ok {
		return nil, nil
	}
	var all []activity.Process
	var rss []float64
	lines := strings.Split(strings.TrimSpace(text), "\n")
	for i := 1; i < len(lines); i++ {
		fields := strings.Fields(lines[i])
		if len(fields) < 4 {
			continue
		}
		c, err1 := parseDecimal(fields[len(fields)-2])
		m, err2 := parseDecimal(fields[len(fields)-1])
		if err1 != nil || err2 != nil {
			continue
		}
		all = append(all, activity.Process{PID: int(parseInt(fields[0])), Name: strings.Join(fields[1:len(fields)-2], " "), Value: c})
		rss = append(rss, m*1024)
	}
	byMemory := make([]activity.Process, len(all))
	for i, p := range all {
		byMemory[i] = activity.Process{PID: p.PID, Name: p.Name, Value: rss[i]}
	}
	return top(all), top(byMemory)
}

// outside: ps prints its decimals in the caller's locale, `69,4` under fr_FR.
func parseDecimal(s string) (float64, error) {
	return strconv.ParseFloat(strings.Replace(s, ",", ".", 1), 64)
}

func top(ps []activity.Process) []activity.Process {
	sorted := append([]activity.Process(nil), ps...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].Value > sorted[j].Value })
	if len(sorted) > activity.Top {
		sorted = sorted[:activity.Top]
	}
	return sorted
}

func network(text string, _ bool) (rx, tx int64) {
	for _, line := range strings.Split(text, "\n") {
		name, counters, ok := strings.Cut(line, ":")
		fields := strings.Fields(counters)
		if !ok || strings.TrimSpace(name) == "lo" || len(fields) < 9 {
			continue
		}
		rx += parseInt(fields[0])
		tx += parseInt(fields[8])
	}
	return rx, tx
}

func parseInt(s string) int64 {
	n, _ := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	return n
}
