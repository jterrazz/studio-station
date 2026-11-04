package activity

import (
	"slices"
	"strconv"
	"time"

	"github.com/jterrazz/studio-station/src/core/finding"
	"github.com/jterrazz/studio-station/src/core/machine"
	"github.com/jterrazz/studio-station/src/core/section"
)

const (
	PressureFor = time.Minute
	SwapFor     = 10 * time.Minute
	BusyFor     = 10 * time.Minute
	BusyCPU     = 95
	LowBattery  = 20
)

func Findings(m machine.Ref, kind machine.Kind, s *Series) []finding.Finding {
	// invariant: SPEC §6.5, read from the series alone: what holds now is
	// judged on the last sample, what must last on the points that span it.
	last, ok := s.Last()
	if !ok {
		return nil
	}
	var out []finding.Finding
	add := func(sev finding.Severity, subject, text, fix string) {
		out = append(out, finding.Finding{Machine: m, Section: section.Activity, Severity: sev, Subject: subject, Text: text, Fix: finding.Fix{Command: fix}})
	}
	on := ""
	if m.On != "" {
		on = " --on " + m.Name
	}
	if last.DiskTotal > 0 {
		free := float64(last.DiskFree) / float64(last.DiskTotal)
		fix := "station disk" + on
		if kind == machine.Server {
			fix = "station machines edit " + m.Name + " --disk <size> --on " + m.On
		}
		text := "disk almost full: " + strconv.Itoa(int(free*100)) + " % free"
		switch {
		case free < 0.05:
			add(finding.Broken, "disk", text, fix)
		case free < 0.10:
			add(finding.NeedsALook, "disk", text, fix)
		}
	}
	points := merged(s)
	if heldFor(points, func(prev, x Sample) bool { return prev.Pressure == PressureCritical && x.Pressure == PressureCritical }) >= PressureFor {
		add(finding.Broken, "memory", "memory pressure critical", "station activity"+on)
	}
	coarse := s.Coarse()
	if heldFor(coarse, func(prev, x Sample) bool { return x.SwapUsed > prev.SwapUsed }) >= SwapFor {
		add(finding.NeedsALook, "swap", "swap growing for 10 min", "station activity"+on)
	}
	switch {
	case last.Thermal >= ThermalCritical:
		add(finding.Broken, "thermal", "running hot: thermal critical", "")
	case last.Thermal == ThermalSerious:
		add(finding.NeedsALook, "thermal", "running hot: thermal serious", "")
	}
	if b := last.Battery; b != nil && b.OnBattery && b.Percent <= LowBattery {
		add(finding.NeedsALook, "battery", "battery low: "+strconv.Itoa(b.Percent)+" % on battery, plug in", "")
	}
	if kind == machine.Server && heldFor(coarse, func(prev, x Sample) bool { return Between(prev, x).CPU >= BusyCPU }) >= BusyFor {
		add(finding.NeedsALook, "cpu", m.Name+" busy: CPU at 95 % or more for 10 min", "station activity"+on)
	}
	return out
}

func merged(s *Series) []Sample {
	all := append(s.Coarse(), s.Fine()...)
	slices.SortStableFunc(all, func(a, b Sample) int { return a.At.Compare(b.At) })
	return slices.CompactFunc(all, func(a, b Sample) bool { return a.At.Equal(b.At) })
}

func heldFor(points []Sample, holds func(prev, x Sample) bool) time.Duration {
	// value: how long, back from the newest point, every point held; a
	// point is judged against the one before it.
	if len(points) < 2 {
		return 0
	}
	newest := points[len(points)-1]
	since := newest.At
	for i := len(points) - 1; i > 0 && holds(points[i-1], points[i]); i-- {
		since = points[i-1].At
	}
	return newest.At.Sub(since)
}
