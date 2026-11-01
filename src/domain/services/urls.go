package services

import "strconv"

const LocalHost = "127.0.0.1"

type URLSet struct {
	Local  string `json:"local"`
	Public string `json:"public"`
}

func (l Lease) isHTTP() bool { return l.HTTP == nil || *l.HTTP }

func URLs(l Lease, publicHost, httpsMode string, serveOn bool) URLSet {
	// invariant: one shape everywhere. An HTTP service, or one not yet detected,
	// carries a scheme so a URL parser reads a host; with Serve off the local URL
	// is the only one there is; a raw TCP service is tailscaled's bytes, no scheme.
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

func ServeArgs(l Lease, httpsMode string, on bool) []string {
	port := strconv.Itoa(l.HostPort)
	switch {
	case !l.isHTTP():
		if on {
			return []string{"--bg", "--tcp=" + port, "tcp://" + LocalHost + ":" + port}
		}
		return []string{"--tcp=" + port, "off"}
	case httpsMode == HTTPSPath:
		p := "--set-path=/" + l.VM + "/" + strconv.Itoa(l.GuestPort)
		if on {
			return []string{"--bg", "--https=443", p, "http://" + LocalHost + ":" + port}
		}
		return []string{"--https=443", p, "off"}
	}
	if on {
		return []string{"--bg", "--https=" + port, "http://" + LocalHost + ":" + port}
	}
	return []string{"--https=" + port, "off"}
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
