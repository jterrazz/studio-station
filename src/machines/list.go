package machines

import (
	"cmp"
	"slices"
	"strconv"
	"strings"

	"github.com/jterrazz/studio-station/src/core/finding"
	"github.com/jterrazz/studio-station/src/core/machine"
	"github.com/jterrazz/studio-station/src/core/section"
)

type Counts struct {
	Machines, Running, Stopped int
}

func Label(f Fleet, h HostView) string {
	if h.Here {
		return "This Mac"
	}
	owner := ""
	for _, o := range f.Hosts {
		if o.Here {
			owner = o.User
		}
	}
	if short, ok := strings.CutPrefix(h.Name, owner+"-"); ok && owner != "" && short != "" {
		return short
	}
	return h.Name
}

func Tally(f Fleet) Counts {
	var c Counts
	for _, h := range f.Hosts {
		c.Machines++
		if h.Reachable {
			c.Running++
		}
		for _, vm := range h.VMs {
			c.Machines++
			switch vm.State {
			case Running:
				c.Running++
			case Stopped, Suspended:
				c.Stopped++
			case Stale, Broken, UnknownState:
			}
		}
	}
	return c
}

func Ordered(vms []VM) []VM {
	out := slices.Clone(vms)
	slices.SortStableFunc(out, func(a, b VM) int { return cmp.Compare(a.Name, b.Name) })
	return out
}

func Behind(h HostView, here string) (fix string, differs bool) {
	if h.Here || h.StationVersion == "" || here == "" || h.StationVersion == here {
		return "", false
	}
	if newer(h.StationVersion, here) {
		return "update station here (scripts/install.sh)", true
	}
	return "update station there (scripts/install.sh)", true
}

func Findings(f Fleet) []finding.Finding {
	here := ""
	for _, h := range f.Hosts {
		if h.Here {
			here = h.StationVersion
		}
	}
	var out []finding.Finding
	add := func(m machine.Ref, sev finding.Severity, subject, text, fix string) {
		out = append(out, finding.Finding{Machine: m, Section: section.Overview, Severity: sev, Subject: subject, Text: text, Fix: finding.Fix{Command: fix}})
	}
	for _, h := range f.Hosts {
		label, mac := Label(f, h), machine.Ref{Name: h.Name}
		on := ""
		if !h.Here {
			on = " --on " + label
		}
		switch {
		case !h.Reachable:
			add(mac, finding.Broken, label, label+" does not answer", "")
		case h.Daemon.Known && !h.Daemon.Running && !h.Daemon.Installed && h.Here:
			add(mac, finding.Broken, label, label+" has no engine set up", "station setup")
		case h.Daemon.Known && !h.Daemon.Running:
			add(mac, finding.Broken, label, label+" has its engine stopped", "station engine start"+on)
		}
		if fix, ok := Behind(h, here); ok && h.Reachable {
			out = append(out, finding.Finding{Machine: mac, Section: section.Overview, Severity: finding.NeedsALook, Subject: label, Text: label + " runs " + h.StationVersion, Fix: finding.Fix{Path: fix}})
		}
		for _, vm := range Ordered(h.VMs) {
			ref := machine.Ref{Name: vm.Name, On: h.Name}
			switch vm.State {
			case Stale:
				add(ref, finding.NeedsALook, vm.Name, vm.Name+" ended unexpectedly", "station machines start "+vm.Name+on)
			case Broken:
				add(ref, finding.Broken, vm.Name, vm.Name+" "+vm.Detail, "station machines delete "+vm.Name+on)
			case Running, Stopped, Suspended, UnknownState:
			}
		}
	}
	return finding.Rank(out)
}

// value: whether release a is past release b, both vMAJOR.MINOR.PATCH with
// anything after the patch ignored; an unreadable one is never newer.
func newer(a, b string) bool {
	pa, pb := release(a), release(b)
	if pa == nil || pb == nil {
		return false
	}
	return slices.Compare(pa, pb) > 0
}

func release(v string) []int {
	core, ok := strings.CutPrefix(v, "v")
	if !ok {
		return nil
	}
	core, _, _ = strings.Cut(core, "-")
	parts := strings.Split(core, ".")
	if len(parts) != 3 {
		return nil
	}
	out := make([]int, 0, 3)
	for _, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			return nil
		}
		out = append(out, n)
	}
	return out
}

type Need struct {
	Broken             bool
	Subject, Text, Fix string
}

func Needs(f Fleet) []Need {
	fs := Findings(f)
	out := make([]Need, 0, len(fs))
	for _, x := range fs {
		out = append(out, Need{Broken: x.Severity == finding.Broken, Subject: x.Subject, Text: x.Text, Fix: x.Fix.Command + x.Fix.Path})
	}
	return out
}
