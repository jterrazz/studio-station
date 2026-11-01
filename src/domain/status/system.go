package status

import (
	"context"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const topProcesses = 5

type systemFacts struct {
	uptime, battery string
	memory          int64
}

func (s *Service) osName(ctx context.Context) string {
	if r := s.ask(ctx, cmd("uname", "-sr"), defaultBudget); r.ok() {
		if name := strings.TrimSpace(r.text()); name != "" {
			return name
		}
	}
	return s.h.Env.GOOS()
}

func (s *Service) systemFacts(ctx context.Context) systemFacts {
	var (
		facts systemFacts
		wg    sync.WaitGroup
	)
	run := func(fn func()) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			fn()
		}()
	}
	if s.mac() {
		run(func() { facts.uptime = s.macUptime(ctx) })
		run(func() { facts.battery = battery(s.ask(ctx, cmd("pmset", "-g", "batt"), defaultBudget)) })
		run(func() {
			facts.memory = parseInt(strings.TrimSpace(s.ask(ctx, cmd("sysctl", "-n", "hw.memsize"), defaultBudget).text()))
		})
	} else {
		run(func() { facts.uptime = linuxUptime(s.ask(ctx, cmd("cat", "/proc/uptime"), defaultBudget)) })
		run(func() { facts.memory = linuxMemory(s.ask(ctx, cmd("cat", "/proc/meminfo"), defaultBudget)) })
	}
	wg.Wait()
	return facts
}

func (s *Service) macUptime(ctx context.Context) string {
	r := s.ask(ctx, cmd("sysctl", "-n", "kern.boottime"), defaultBudget)
	_, after, ok := strings.Cut(r.text(), "sec = ")
	if !ok {
		return ""
	}
	boot, _, _ := strings.Cut(after, ",")
	sec := parseInt(strings.TrimSpace(boot))
	if sec <= 0 {
		return ""
	}
	return humanDuration(s.h.Clock.Now().Sub(time.Unix(sec, 0)))
}

func linuxUptime(r reply) string {
	field, _, _ := strings.Cut(strings.TrimSpace(r.text()), " ")
	seconds, err := strconv.ParseFloat(field, 64)
	if !r.ok() || err != nil {
		return ""
	}
	return humanDuration(time.Duration(seconds * float64(time.Second)))
}

func humanDuration(d time.Duration) string {
	minutes := int(d.Minutes())
	days, hours := minutes/(24*60), minutes/60%24
	switch {
	case days > 0:
		return strconv.Itoa(days) + "d " + strconv.Itoa(hours) + "h"
	case hours > 0:
		return strconv.Itoa(hours) + "h " + strconv.Itoa(minutes%60) + "m"
	}
	return strconv.Itoa(minutes) + "m"
}

// outside: pmset prints "-InternalBattery-0 (id=...)<TAB>92%; charging; 1:23 remaining";
// a desktop prints no such line, and then there is no battery to report.
func battery(r reply) string {
	if !r.ok() {
		return ""
	}
	for _, line := range strings.Split(r.text(), "\n") {
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

func linuxMemory(r reply) int64 {
	for _, line := range strings.Split(r.text(), "\n") {
		if rest, ok := strings.CutPrefix(line, "MemTotal:"); ok {
			fields := strings.Fields(rest)
			if len(fields) > 0 {
				return parseInt(fields[0]) * 1024
			}
		}
	}
	return 0
}

func (s *Service) Sample(ctx context.Context) (Sample, error) {
	if err := ctx.Err(); err != nil {
		return Sample{}, err
	}
	sample := Sample{At: s.h.Clock.Now().UTC()}
	var wg sync.WaitGroup
	run := func(fn func()) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			fn()
		}()
	}
	if s.mac() {
		run(func() {
			sample.CPU = parseMacProcesses(s.ask(ctx, cmd("ps", "-arcwwwxo", "pid,%cpu,comm"), defaultBudget))
		})
		run(func() {
			sample.Memory = scaleKB(parseMacProcesses(s.ask(ctx, cmd("ps", "-amcwwwxo", "pid,rss,comm"), defaultBudget)))
		})
		run(func() { sample.NetRx, sample.NetTx = macNetwork(s.ask(ctx, cmd("netstat", "-ib"), defaultBudget)) })
		run(func() {
			sample.GPU = gpuLoad(s.ask(ctx, cmd("ioreg", "-r", "-d", "1", "-c", "IOAccelerator"), defaultBudget))
		})
	} else {
		run(func() {
			sample.CPU, sample.Memory = linuxProcesses(s.ask(ctx, cmd("ps", "-eo", "pid,comm,%cpu,rss"), defaultBudget))
		})
		run(func() {
			sample.NetRx, sample.NetTx = linuxNetwork(s.ask(ctx, cmd("cat", "/proc/net/dev"), defaultBudget))
		})
	}
	wg.Wait()
	if sample.CPU == nil {
		sample.CPU = []Process{}
	}
	if sample.Memory == nil {
		sample.Memory = []Process{}
	}
	return sample, nil
}

// outside: macOS ps prints a header, then "pid value command" with a command that
// may hold spaces ("Google Chrome Helper"), already sorted by the flag asked.
func parseMacProcesses(r reply) []Process {
	var out []Process
	if !r.ok() {
		return out
	}
	lines := strings.Split(strings.TrimSpace(r.text()), "\n")
	for i := 1; i < len(lines) && len(out) < topProcesses; i++ {
		fields := strings.Fields(lines[i])
		if len(fields) < 3 {
			continue
		}
		value, err := parseDecimal(fields[1])
		if err != nil {
			continue
		}
		out = append(out, Process{PID: int(parseInt(fields[0])), Name: strings.Join(fields[2:], " "), Value: value})
	}
	return out
}

func scaleKB(ps []Process) []Process {
	for i := range ps {
		ps[i].Value *= 1024
	}
	return ps
}

func linuxProcesses(r reply) (cpu, memory []Process) {
	if !r.ok() {
		return nil, nil
	}
	var all []Process
	var rss []float64
	lines := strings.Split(strings.TrimSpace(r.text()), "\n")
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
		all = append(all, Process{PID: int(parseInt(fields[0])), Name: strings.Join(fields[1:len(fields)-2], " "), Value: c})
		rss = append(rss, m*1024)
	}
	cpu = top(all)
	byMemory := make([]Process, len(all))
	for i, p := range all {
		byMemory[i] = Process{PID: p.PID, Name: p.Name, Value: rss[i]}
	}
	memory = top(byMemory)
	return cpu, memory
}

// outside: ps prints its decimals in the caller's locale, `69,4` under fr_FR;
// it is not run under C, which would escape every non-ASCII command name.
func parseDecimal(s string) (float64, error) {
	return strconv.ParseFloat(strings.Replace(s, ",", ".", 1), 64)
}

func top(ps []Process) []Process {
	sorted := append([]Process(nil), ps...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].Value > sorted[j].Value })
	if len(sorted) > topProcesses {
		sorted = sorted[:topProcesses]
	}
	return sorted
}

// outside: the first en0 line carrying <Link#> holds the interface's raw counters.
func macNetwork(r reply) (rx, tx int64) {
	for _, line := range strings.Split(r.text(), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 10 || !strings.HasPrefix(fields[0], "en0") || !strings.HasPrefix(fields[2], "<Link") {
			continue
		}
		return parseInt(fields[6]), parseInt(fields[9])
	}
	return 0, 0
}

func linuxNetwork(r reply) (rx, tx int64) {
	for _, line := range strings.Split(r.text(), "\n") {
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

func gpuLoad(r reply) float64 {
	_, after, ok := strings.Cut(r.text(), `"Device Utilization %"=`)
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
