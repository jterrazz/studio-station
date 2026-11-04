package services

import (
	"strconv"

	"github.com/jterrazz/studio-station/src/ports"
)

const LocalHost = "127.0.0.1"

type URLSet struct {
	Local  string `json:"local"`
	Public string `json:"public"`
}

func (l Lease) isHTTP() bool { return l.HTTP == nil || *l.HTTP }

func URLs(l Lease, publicHost, httpsMode string, serveOn bool) URLSet {
	// invariant: one shape everywhere. An HTTP service, or one not yet detected,
	// carries a scheme so a URL parser reads a host; with Serve off the local URL
	// is the only one there is; a raw TCP service is the tailnet's bytes, no scheme.
	port := strconv.Itoa(l.HostPort)
	u := URLSet{Local: LocalHost + ":" + port}
	if l.isHTTP() {
		u.Local = "http://" + u.Local
	}
	u.Public = u.Local
	if !serveOn || publicHost == "" {
		return u
	}
	switch {
	case !l.isHTTP():
		u.Public = publicHost + ":" + port
	case httpsMode == HTTPSPath:
		u.Public = "https://" + publicHost + "/" + l.VM + "/" + strconv.Itoa(l.GuestPort) + "/"
	default:
		u.Public = "https://" + publicHost + ":" + port
	}
	return u
}

func ShareOf(l Lease, httpsMode string) ports.Share {
	s := ports.Share{Port: l.HostPort, Reserved: l.HostPort, Scheme: ports.SchemeHTTPS}
	switch {
	case !l.isHTTP():
		s.Scheme = ports.SchemeTCP
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
