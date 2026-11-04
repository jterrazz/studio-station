package linux

import (
	"context"
	"errors"
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
	now := m.clock.Now().UTC()
	s := activity.Sample{At: now, Thermal: activity.ThermalUnknown}
	files := map[string]string{"stat": "", "loadavg": "", "meminfo": "", "net/dev": "", "uptime": ""}
	var mu sync.Mutex
	var wg sync.WaitGroup
	for name := range files {
		wg.Add(1)
		go func() {
			defer wg.Done()
			text, _ := m.ask(ctx, "cat", "/proc/"+name)
			mu.Lock()
			files[name] = text
			mu.Unlock()
		}()
	}
	wg.Wait()
	s.CPUBusy, s.CPUTotal, s.CPUs = ticks(files["stat"])
	if fields := strings.Fields(files["loadavg"]); len(fields) >= 3 {
		for i := range s.Load {
			s.Load[i], _ = strconv.ParseFloat(fields[i], 64)
		}
	}
	memory := meminfo(files["meminfo"])
	s.MemTotal = memory["MemTotal"]
	if avail, ok := memory["MemAvailable"]; ok {
		s.MemUsed = max(s.MemTotal-avail, 0)
	}
	s.SwapUsed = max(memory["SwapTotal"]-memory["SwapFree"], 0)
	s.NetIn, s.NetOut = network(files["net/dev"])
	field, _, _ := strings.Cut(strings.TrimSpace(files["uptime"]), " ")
	if seconds, err := strconv.ParseFloat(field, 64); err == nil && seconds > 0 {
		s.Boot = now.Add(-time.Duration(seconds * float64(time.Second))).Truncate(time.Second)
	}
	return s, nil
}

func (m *Meter) Top(ctx context.Context) (activity.Busiest, error) {
	if err := ctx.Err(); err != nil {
		return activity.Busiest{}, err
	}
	cpu, memory := processes(m.ask(ctx, "ps", "-eo", "pid,comm,%cpu,rss"))
	if cpu == nil {
		cpu, memory = []activity.Process{}, []activity.Process{}
	}
	return activity.Busiest{CPU: top(cpu), Memory: top(memory)}, nil
}

func (m *Meter) Process(ctx context.Context, pid int) (activity.Usage, error) {
	if err := ctx.Err(); err != nil {
		return activity.Usage{}, err
	}
	cpu, memory := processes(m.ask(ctx, "ps", "-eo", "pid,comm,%cpu,rss"))
	for i, p := range cpu {
		if p.PID == pid {
			return activity.Usage{CPU: p.Value, Memory: int64(memory[i].Value)}, nil
		}
	}
	return activity.Usage{}, ErrNoProcess
}

var ErrNoProcess = errors.New("no such process")

// outside: the first line of /proc/stat sums every CPU's ticks: user nice
// system idle iowait irq softirq steal; one cpuN line follows per CPU.
func ticks(text string) (busy, total uint64, cpus int) {
	for _, line := range strings.Split(text, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 5 || !strings.HasPrefix(fields[0], "cpu") {
			continue
		}
		if fields[0] != "cpu" {
			cpus++
			continue
		}
		for i, f := range fields[1:min(len(fields), 9)] {
			n, _ := strconv.ParseUint(f, 10, 64)
			total += n
			if i != 3 && i != 4 {
				busy += n
			}
		}
	}
	return busy, total, cpus
}

func meminfo(text string) map[string]int64 {
	out := map[string]int64{}
	for _, line := range strings.Split(text, "\n") {
		key, rest, ok := strings.Cut(line, ":")
		if fields := strings.Fields(rest); ok && len(fields) > 0 {
			out[key] = parseInt(fields[0]) * 1024
		}
	}
	return out
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
	return all, byMemory
}

// outside: ps prints its decimals in the caller's locale, `69,4` under fr_FR.
func parseDecimal(s string) (float64, error) {
	return strconv.ParseFloat(strings.Replace(s, ",", ".", 1), 64)
}

func top(ps []activity.Process) []activity.Process {
	sorted := append([]activity.Process{}, ps...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].Value > sorted[j].Value })
	if len(sorted) > activity.Top {
		sorted = sorted[:activity.Top]
	}
	return sorted
}

func network(text string) (rx, tx uint64) {
	for _, line := range strings.Split(text, "\n") {
		name, counters, ok := strings.Cut(line, ":")
		fields := strings.Fields(counters)
		if !ok || strings.TrimSpace(name) == "lo" || len(fields) < 9 {
			continue
		}
		rx += uint64(max(parseInt(fields[0]), 0))
		tx += uint64(max(parseInt(fields[8]), 0))
	}
	return rx, tx
}

func parseInt(s string) int64 {
	n, _ := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	return n
}
