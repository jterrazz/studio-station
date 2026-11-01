package services

import (
	"slices"
	"strconv"
	"strings"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/domain/settings"
)

const (
	HTTPSPort = "port"
	HTTPSPath = "path"
	HTTPSOff  = "off"
)

type Policy struct {
	Low, High, MaxPerVM, MinPort int
	Ignored                      []int
	HTTPS                        string
}

func PolicyOf(s *settings.Services) Policy {
	d := catalog.ServicePolicy
	p := Policy{Low: d.RangeLow, High: d.RangeHigh, MaxPerVM: d.MaxPerVM, MinPort: d.MinPort, Ignored: d.Ignored, HTTPS: d.HTTPS}
	if s == nil {
		return p
	}
	if lo, hi, ok := strings.Cut(s.Range, "-"); ok {
		l, errL := strconv.Atoi(strings.TrimSpace(lo))
		h, errH := strconv.Atoi(strings.TrimSpace(hi))
		if errL == nil && errH == nil && l > 0 && h >= l && h < 65536 {
			p.Low, p.High = l, h
		}
	}
	if s.MaxPerVM > 0 {
		p.MaxPerVM = s.MaxPerVM
	}
	if s.MinPort > 0 {
		p.MinPort = s.MinPort
	}
	if s.IgnoredPorts != nil {
		p.Ignored = s.IgnoredPorts
	}
	switch s.HTTPS {
	case HTTPSPort, HTTPSPath, HTTPSOff:
		p.HTTPS = s.HTTPS
	}
	return p
}

func (p Policy) Ignores(port int) bool { return port < p.MinPort || slices.Contains(p.Ignored, port) }

func (p Policy) InRange(port int) bool { return port >= p.Low && port <= p.High }

func ProfileOf(port int) (catalog.Profile, bool) {
	for _, pr := range catalog.Profiles {
		if pr.Port == port {
			return pr, true
		}
	}
	return catalog.Profile{}, false
}
