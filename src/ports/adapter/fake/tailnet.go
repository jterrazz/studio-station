package fake

import (
	"context"
	"slices"
	"strconv"
	"sync"

	"github.com/jterrazz/studio-station/src/ports"
)

type Tailnet struct {
	mu     sync.Mutex
	self   ports.Self
	shares []ports.Share
}

func NewTailnet(self ports.Self) *Tailnet { return &Tailnet{self: self} }

func (t *Tailnet) Self(context.Context) (ports.Self, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.self, nil
}

func (t *Tailnet) Ready(context.Context) (ports.SharingState, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.self.Sharing, nil
}

func same(a, b ports.Share) bool {
	return a.Port == b.Port && a.Scheme == b.Scheme && (a.Path == b.Path || !a.Mounted() && !b.Mounted())
}

func (t *Tailnet) Share(_ context.Context, s ports.Share) (string, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	scheme := "http://"
	if s.Scheme == ports.SchemeTCP {
		scheme = ""
	}
	s.Target = scheme + "127.0.0.1:" + strconv.Itoa(s.Reserved)
	if !s.Mounted() {
		s.Path = ""
		if s.Scheme == ports.SchemeHTTPS {
			s.Path = "/"
		}
	}
	t.shares = append(slices.DeleteFunc(t.shares, func(o ports.Share) bool { return same(o, s) }), s)
	host := t.self.Sharing.Host
	switch {
	case s.Scheme == ports.SchemeTCP:
		return host + ":" + strconv.Itoa(s.Port), nil
	case s.Mounted():
		return "https://" + host + s.Path + "/", nil
	}
	return "https://" + host + ":" + strconv.Itoa(s.Port), nil
}

func (t *Tailnet) Unshare(_ context.Context, s ports.Share) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.shares = slices.DeleteFunc(t.shares, func(o ports.Share) bool { return same(o, s) })
	return nil
}

func (t *Tailnet) Shared(context.Context) ([]ports.Share, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	out := slices.Clone(t.shares)
	slices.SortFunc(out, func(a, b ports.Share) int {
		if a.Port != b.Port {
			return a.Port - b.Port
		}
		if a.Path < b.Path {
			return -1
		}
		if a.Path > b.Path {
			return 1
		}
		return 0
	})
	return out, nil
}

func (t *Tailnet) Warm(context.Context) error { return nil }
