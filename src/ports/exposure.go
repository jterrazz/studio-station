package ports

import (
	"net/netip"
	"slices"
	"strconv"
	"strings"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/core/finding"
	"github.com/jterrazz/studio-station/src/core/machine"
	"github.com/jterrazz/studio-station/src/core/section"
)

type Class string

const (
	Private Class = "private"
	Public  Class = "public"
)

type Placement string // value: where a machine sits: a Mac, a VM behind its Mac (the default), a VM bridged onto its Mac's network.

const (
	OnMac     Placement = "mac"
	BehindMac Placement = "behind"
	Bridged   Placement = "bridged"
)

type Reach int // value: who reaches an address a port listens on, narrowest first.

const (
	ThisMachine Reach = iota
	Tailnet
	Outside
)

var tailnetAddresses = []netip.Prefix{netip.MustParsePrefix("100.64.0.0/10"), netip.MustParsePrefix("fd7a:115c:a1e0::/48")}

func ReachOf(bind string, at Placement) Reach {
	// invariant: SPEC 5.6.1: loopback reaches this machine only; the tailnet
	// address, the tailnet; every interface or a LAN address reaches outside
	// the tailnet, except on a VM behind its Mac, which only its Mac and its
	// own tailnet device reach.
	addr := strings.Trim(bind, "[]")
	if i := strings.IndexByte(addr, '%'); i >= 0 {
		addr = addr[:i]
	}
	if addr == "localhost" {
		return ThisMachine
	}
	ip, err := netip.ParseAddr(addr)
	switch {
	case err == nil && ip.IsLoopback():
		return ThisMachine
	case err == nil && slices.ContainsFunc(tailnetAddresses, func(p netip.Prefix) bool { return p.Contains(ip) }):
		return Tailnet
	case at == BehindMac:
		return Tailnet
	}
	return Outside
}

type Machine struct {
	Ref machine.Ref
	At  Placement
	// value: the machine has its own device on the tailnet.
	Device    bool
	Listening []Listener
}

type Port struct {
	Machine machine.Ref
	Port    int
	Process string
	// value: every address it listens on, in the order the system reported them.
	Binds    []string
	Reach    Reach
	Device   bool
	Declared *Declaration
}

func (p Port) Key() string { return p.Machine.Name + ":" + strconv.Itoa(p.Port) }

func (p Port) Red() bool { return p.Reach == Outside && p.Declared == nil }

func (p Port) Class() Class {
	// invariant: two classes: a port reachable outside the tailnet is public,
	// declared or not; only an undeclared one is red.
	if p.Declared != nil || p.Reach == Outside {
		return Public
	}
	return Private
}

func (p Port) What() string {
	if k, ok := catalog.KnownPortOf(p.Port, p.Process); ok {
		return k.What
	}
	return p.Process
}

func Classify(m Machine, decl []Declaration) []Port {
	// invariant: one port per number, its reach the widest of its addresses,
	// declared when a declaration names this machine and port; the system's
	// own listeners are left out unless the catalogue knows the port.
	var out []Port
	for _, l := range m.Listening {
		if slices.Contains(catalog.SystemListeners, l.Process) {
			if _, known := catalog.KnownPortOf(l.Port, l.Process); !known {
				continue
			}
		}
		i := slices.IndexFunc(out, func(p Port) bool { return p.Port == l.Port })
		if i < 0 {
			out = append(out, Port{Machine: m.Ref, Port: l.Port, Process: l.Process, Device: m.Device, Reach: ThisMachine})
			i = len(out) - 1
		}
		p := &out[i]
		if !slices.Contains(p.Binds, l.Bind) {
			p.Binds = append(p.Binds, l.Bind)
		}
		p.Reach = max(p.Reach, ReachOf(l.Bind, m.At))
	}
	for i := range out {
		for _, d := range decl {
			if d.Machine == m.Ref.Name && d.Port == out[i].Port {
				out[i].Declared = &d
			}
		}
	}
	slices.SortStableFunc(out, func(a, b Port) int { return a.Port - b.Port })
	return out
}

func Exposed(ps []Port) []finding.Finding {
	// invariant: every port reachable outside the tailnet and not declared
	// public is a broken finding of the Ports section, detection over
	// declaration.
	var out []finding.Finding
	for _, p := range ps {
		if !p.Red() {
			continue
		}
		f := finding.Finding{Machine: p.Machine, Section: section.Ports, Severity: finding.Broken, Subject: p.Key(),
			Text: "port " + strconv.Itoa(p.Port) + " is " + catalog.ReachableOutside}
		if k, ok := catalog.KnownPortOf(p.Port, p.Process); ok {
			f.Fix.Path = k.Off
		}
		out = append(out, f)
	}
	return out
}
