package contracttest

import (
	"context"
	"strconv"
	"strings"
	"testing"

	"github.com/jterrazz/studio-station/src/ports"
	"github.com/jterrazz/studio-station/src/ports/contract"
)

func Tailnet(t *testing.T, open func(t *testing.T) contract.Tailnet) {
	// value: open answers a node signed in to its tailnet, sharing on, sharing
	// nothing yet.
	ctx := context.Background()
	web := ports.Share{Port: 40001, Reserved: 40001, Scheme: ports.SchemeHTTPS}
	raw := ports.Share{Port: 40002, Reserved: 40002, Scheme: ports.SchemeTCP}
	mounted := ports.Share{Port: 443, Path: "/linux-1/5173", Reserved: 40003, Scheme: ports.SchemeHTTPS}

	t.Run("a node with sharing on names the host its shares answer at", func(t *testing.T) {
		tn := open(t)
		ready, err := tn.Ready(ctx)
		if err != nil || !ready.On || ready.Host == "" {
			t.Fatalf("Ready = %+v, %v; want sharing on at a host", ready, err)
		}
		self, err := tn.Self(ctx)
		if err != nil || self.Sharing != ready || self.DNSName != ready.Host {
			t.Fatalf("Self = %+v, %v; want the sharing Ready reported, %+v", self, err, ready)
		}
	})

	t.Run("a shared port is listed where it is served, carried from this Mac's loopback only", func(t *testing.T) {
		tn := open(t)
		ready, _ := tn.Ready(ctx)
		want := map[ports.Share]string{
			web:     "https://" + ready.Host + ":40001",
			raw:     ready.Host + ":40002",
			mounted: "https://" + ready.Host + "/linux-1/5173/",
		}
		for _, s := range []ports.Share{web, raw, mounted} {
			url, err := tn.Share(ctx, s)
			if err != nil || url != want[s] {
				t.Fatalf("Share(%+v) = %q, %v; want %q", s, url, err, want[s])
			}
		}
		shared, err := tn.Shared(ctx)
		if err != nil || len(shared) != 3 {
			t.Fatalf("Shared = %+v, %v; want the three shares", shared, err)
		}
		for _, s := range []ports.Share{web, raw, mounted} {
			got, ok := find(shared, s)
			if !ok {
				t.Fatalf("Shared = %+v; want %+v listed", shared, s)
			}
			if !strings.Contains(got.Target, "127.0.0.1:"+strconv.Itoa(s.Reserved)) || strings.Contains(got.Target, "0.0.0.0") {
				t.Fatalf("share %+v is carried from %q; want 127.0.0.1:%d", s, got.Target, s.Reserved)
			}
		}
	})

	t.Run("an unshared port leaves the others served", func(t *testing.T) {
		tn := open(t)
		for _, s := range []ports.Share{web, raw, mounted} {
			if _, err := tn.Share(ctx, s); err != nil {
				t.Fatalf("Share(%+v): %v", s, err)
			}
		}
		if err := tn.Unshare(ctx, web); err != nil {
			t.Fatalf("Unshare: %v", err)
		}
		shared, err := tn.Shared(ctx)
		if _, still := find(shared, web); err != nil || still || len(shared) != 2 {
			t.Fatalf("Shared after unsharing %+v = %+v, %v; want the two others", web, shared, err)
		}
	})

	t.Run("warming the certificate of a node sharing on succeeds", func(t *testing.T) {
		tn := open(t)
		if _, err := tn.Ready(ctx); err != nil {
			t.Fatalf("Ready: %v", err)
		}
		if err := tn.Warm(ctx); err != nil {
			t.Fatalf("Warm: %v", err)
		}
	})
}

func find(shared []ports.Share, s ports.Share) (ports.Share, bool) {
	for _, got := range shared {
		if got.Port == s.Port && got.Scheme == s.Scheme && got.Mounted() == s.Mounted() && (!s.Mounted() || got.Path == s.Path) {
			return got, true
		}
	}
	return ports.Share{}, false
}
