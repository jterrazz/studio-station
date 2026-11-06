package ports

import (
	"errors"
	"strconv"
	"time"

	"github.com/jterrazz/studio-station/src/catalog"
)

var ErrDaemonDown = errors.New("the engine is not running")

const (
	ServeOn  = "on"
	ServeOff = "off"
)

type Exposure struct {
	On         bool
	PublicHost string
	// value: why nothing is shared when it is off.
	Off string
}

func ExposureOf(serveOn bool, dnsName string) Exposure {
	if !serveOn || dnsName == "" {
		return Exposure{PublicHost: LocalHost, Off: catalog.ServeOffDetail}
	}
	return Exposure{On: true, PublicHost: dnsName}
}

func (e Exposure) Word() string {
	if e.On {
		return ServeOn
	}
	return ServeOff
}

func ViewOf(l Reservation, e Exposure, httpsMode string, phase Phase) ServiceView {
	v := ServiceView{
		VM: l.VM, GuestPort: l.GuestPort, HostPort: l.HostPort, State: phase, Pinned: l.Pinned, Profile: l.Profile,
		HTTP: l.HTTP != nil && *l.HTTP, URLs: URLs(l, e.PublicHost, httpsMode, e.On),
	}
	if !l.LastSeenAt.IsZero() {
		seen := l.LastSeenAt
		v.LastSeenAt = &seen
	}
	if !e.On {
		v.Detail = e.Off
	}
	return v
}

func JoinDetail(a, b string) string {
	switch {
	case a == "":
		return b
	case b == "":
		return a
	}
	return a + ", " + b
}

func ParsePort(text string) (int, error) {
	n, err := strconv.Atoi(text)
	if err != nil || n < 1 || n > 65535 {
		return 0, &BadPortError{Value: text}
	}
	return n, nil
}

func NotFoundAt(t Target) error { return NotFound(t.VM, t.Port) }

const LeasesSchema = "station.services/v1"

func Find(leases []Reservation, vm string, port int) (found Reservation, index int) {
	for i, l := range leases {
		if l.VM == vm && l.GuestPort == port {
			return l, i
		}
	}
	return Reservation{}, -1
}

func Without(leases []Reservation, keep func(Reservation) bool) (kept, gone []Reservation) {
	for _, l := range leases {
		if keep(l) {
			kept = append(kept, l)
		} else {
			gone = append(gone, l)
		}
	}
	return kept, gone
}

type Event int

const (
	EvListen Event = iota
	EvUnlisten
	EvTick
	EvVMStopped
	EvDeleted
	EvReleased
)

type Action int

const (
	ActPublish Action = iota
	ActUnpublish
	ActRelease
)

type State struct {
	Phase      Phase
	Since      time.Time
	Pinned     bool
	LastSeenAt time.Time
}

func Next(st State, ev Event, now time.Time) (State, []Action) {
	// invariant: the lifecycle of one service (N5): a stop of listening unpublishes
	// only after the grace, a re-listen inside it keeps everything; a lease outlives
	// its listener and idles out after 7 days unless pinned.
	switch ev {
	case EvListen:
		st.LastSeenAt = now
		switch st.Phase {
		case PhaseClosing:
			st.Phase = PhasePublished
			return st, nil
		case PhasePublished:
			return st, nil
		case PhaseNone, PhaseWaiting, PhaseRefused, PhaseGone, PhaseUnknown:
		}
		st.Phase, st.Since = PhasePublished, now
		return st, []Action{ActPublish}
	case EvUnlisten:
		if st.Phase == PhasePublished {
			st.Phase, st.Since = PhaseClosing, now
		}
		return st, nil
	case EvTick:
		switch {
		case st.Phase == PhaseClosing && now.Sub(st.Since) >= catalog.ServicePolicy.Grace:
			st.Phase, st.Since = PhaseWaiting, now
			return st, []Action{ActUnpublish}
		case st.Phase == PhaseWaiting && !st.Pinned && !st.LastSeenAt.IsZero() && now.Sub(st.LastSeenAt) > catalog.ServicePolicy.Idle:
			st.Phase = PhaseGone
			return st, []Action{ActRelease}
		}
		return st, nil
	case EvVMStopped:
		if st.Phase == PhasePublished || st.Phase == PhaseClosing {
			st.Phase, st.Since = PhaseWaiting, now
			return st, []Action{ActUnpublish}
		}
		if st.Phase == PhaseNone {
			st.Phase = PhaseWaiting
		}
		return st, nil
	case EvDeleted, EvReleased:
		was := st.Phase
		st.Phase = PhaseGone
		if was == PhasePublished || was == PhaseClosing {
			return st, []Action{ActUnpublish, ActRelease}
		}
		return st, []Action{ActRelease}
	}
	return st, nil
}

const LocalHost = "127.0.0.1"

func isHTTP(l Reservation) bool { return l.HTTP == nil || *l.HTTP }

func URLs(l Reservation, publicHost, httpsMode string, serveOn bool) URLSet {
	// invariant: one shape everywhere. An HTTP service, or one not yet detected,
	// carries a scheme so a URL parser reads a host; with Serve off the local URL
	// is the only one there is; a raw TCP service is the tailnet's bytes, no scheme.
	port := strconv.Itoa(l.HostPort)
	u := URLSet{Local: LocalHost + ":" + port}
	if isHTTP(l) {
		u.Local = "http://" + u.Local
	}
	u.Public = u.Local
	if !serveOn || publicHost == "" {
		return u
	}
	switch {
	case !isHTTP(l):
		u.Public = publicHost + ":" + port
	case httpsMode == HTTPSPath:
		u.Public = "https://" + publicHost + "/" + l.VM + "/" + strconv.Itoa(l.GuestPort) + "/"
	default:
		u.Public = "https://" + publicHost + ":" + port
	}
	return u
}

func ShareOf(l Reservation, httpsMode string) Share {
	s := Share{Port: l.HostPort, Reserved: l.HostPort, Scheme: SchemeHTTPS}
	switch {
	case !isHTTP(l):
		s.Scheme = SchemeTCP
	case httpsMode == HTTPSPath:
		s.Port, s.Path = 443, "/"+l.VM+"/"+strconv.Itoa(l.GuestPort)
	}
	return s
}

func Dialed(ip string) string {
	// invariant: guest-reported wildcards map to the loopback of their family, so
	// a tool bound to 127.0.0.1 in the VM is reached without --host.
	switch ip {
	case "", "0.0.0.0":
		return "127.0.0.1"
	case "::":
		return "::1"
	}
	return ip
}

type BadPortError struct{ Value string }

func (e *BadPortError) Error() string { return "\"" + e.Value + "\" is not a port" }

type IgnoredPortError struct{ Port, MinPort int }

func (e *IgnoredPortError) Error() string {
	return "port " + strconv.Itoa(e.Port) + " is never shared"
}

type HostAwayError struct{ Host string }

func (e *HostAwayError) Error() string {
	if e.Host == "" {
		return "no host has connected to this VM yet"
	}
	return e.Host + " is not connected to this VM"
}

func (e *HostAwayError) Hint() string {
	if e.Host == "" {
		return "on the VM's host: station setup"
	}
	return "on " + e.Host + ": station setup"
}

type UnknownVMError struct{ Name string }

func (e *UnknownVMError) Error() string { return "unknown VM \"" + e.Name + "\"" }
