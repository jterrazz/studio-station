package fake

import (
	"context"
	"slices"
	"strconv"

	"github.com/jterrazz/studio-station/src/ports"
	"github.com/jterrazz/studio-station/src/world"
)

type Tailnet struct {
	// invariant: one Mac's node of the world; a Mac with no node is signed
	// out, and shares nothing.
	w   *world.World
	mac string
}

func TailnetOf(w *world.World, mac string) *Tailnet { return &Tailnet{w: w, mac: mac} }

func NewTailnet(self ports.Self) *Tailnet {
	w := alone()
	w.Mac(here).Tailnet = &world.Node{Self: self}
	return TailnetOf(w, here)
}

func (t *Tailnet) node() *world.Node {
	n := t.w.Mac(t.mac).Tailnet
	if n == nil {
		return &world.Node{}
	}
	return n
}

func (t *Tailnet) Self(context.Context) (ports.Self, error) {
	t.w.Lock()
	defer t.w.Unlock()
	return t.node().Self, nil
}

func (t *Tailnet) Ready(context.Context) (ports.SharingState, error) {
	t.w.Lock()
	defer t.w.Unlock()
	return t.node().Self.Sharing, nil
}

func same(a, b ports.Share) bool {
	return a.Port == b.Port && a.Scheme == b.Scheme && (a.Path == b.Path || !a.Mounted() && !b.Mounted())
}

func (t *Tailnet) Share(_ context.Context, s ports.Share) (string, error) {
	t.w.Lock()
	defer t.w.Unlock()
	if t.w.Fails("Tailnet.Share") {
		return "", errScripted
	}
	n := t.w.Mac(t.mac).Tailnet
	if n == nil || !n.Self.Sharing.On {
		return "", errSharingOff
	}
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
	n.Shares = append(slices.DeleteFunc(n.Shares, func(o ports.Share) bool { return same(o, s) }), s)
	host := n.Self.Sharing.Host
	switch {
	case s.Scheme == ports.SchemeTCP:
		return host + ":" + strconv.Itoa(s.Port), nil
	case s.Mounted():
		return "https://" + host + s.Path + "/", nil
	}
	return "https://" + host + ":" + strconv.Itoa(s.Port), nil
}

func (t *Tailnet) Unshare(_ context.Context, s ports.Share) error {
	t.w.Lock()
	defer t.w.Unlock()
	if n := t.w.Mac(t.mac).Tailnet; n != nil {
		n.Shares = slices.DeleteFunc(n.Shares, func(o ports.Share) bool { return same(o, s) })
	}
	return nil
}

func (t *Tailnet) Shared(context.Context) ([]ports.Share, error) {
	t.w.Lock()
	defer t.w.Unlock()
	out := slices.Clone(t.node().Shares)
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

func (t *Tailnet) Warm(context.Context) error {
	// invariant: a pending certificate is not issued for the whole command:
	// the world changes only between two commands, and its clock is not the
	// deadline's, so the wait is the world's alone.
	if t.pending() {
		select {}
	}
	return nil
}

func (t *Tailnet) pending() bool {
	t.w.Lock()
	defer t.w.Unlock()
	return t.node().CertificatePending
}
