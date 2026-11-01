package services

import "strconv"

type URLSet struct {
	Local   string `json:"local"`
	Tailnet string `json:"tailnet"`
	HTTPS   string `json:"https"`
}

func (l Lease) isHTTP() bool { return l.HTTP == nil || *l.HTTP }

func URLs(l Lease, publicHost, httpsMode string) URLSet {
	// invariant: one shape everywhere; the public URL is the https one when the
	// service is HTTP (or not yet known) and https is on, else the tailnet one.
	port := strconv.Itoa(l.HostPort)
	u := URLSet{Local: "127.0.0.1:" + port}
	if l.HTTP != nil && *l.HTTP {
		u.Local = "http://" + u.Local
	}
	if publicHost == "" {
		return u
	}
	u.Tailnet = publicHost + ":" + port
	if !l.isHTTP() {
		return u
	}
	switch httpsMode {
	case HTTPSPort:
		u.HTTPS = "https://" + publicHost + ":" + port
	case HTTPSPath:
		u.HTTPS = "https://" + publicHost + "/" + l.VM + "/" + strconv.Itoa(l.GuestPort) + "/"
	}
	return u
}

func (u URLSet) Public() string {
	if u.HTTPS != "" {
		return u.HTTPS
	}
	if u.Tailnet != "" {
		return u.Tailnet
	}
	return u.Local
}

func ServeArgs(l Lease, httpsMode string, on bool) []string {
	port := strconv.Itoa(l.HostPort)
	target := "http://127.0.0.1:" + port
	switch httpsMode {
	case HTTPSPort:
		if on {
			return []string{"--bg", "--https=" + port, target}
		}
		return []string{"--https=" + port, "off"}
	case HTTPSPath:
		p := "--set-path=/" + l.VM + "/" + strconv.Itoa(l.GuestPort)
		if on {
			return []string{"--bg", "--https=443", p, target}
		}
		return []string{"--https=443", p, "off"}
	}
	return nil
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
