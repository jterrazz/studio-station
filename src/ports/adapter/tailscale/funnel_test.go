package tailscale

import (
	"context"
	"slices"
	"testing"

	"github.com/jterrazz/studio-station/src/platform/local"
)

// value: `tailscale serve status --json` as recorded on a node with Funnel
// on 443 and 8443, and 10000 served on the tailnet only.
const funnelStatus = `{"TCP":{"443":{"HTTPS":true},"8443":{"HTTPS":true},"10000":{"TCPForward":"127.0.0.1:10000"}},
"Web":{"jterrazz-mac-mini.tail1234.ts.net:443":{"Handlers":{"/":{"Proxy":"http://127.0.0.1:3000"}}},
"jterrazz-mac-mini.tail1234.ts.net:8443":{"Handlers":{"/":{"Proxy":"http://127.0.0.1:8080"}}}},
"AllowFunnel":{"jterrazz-mac-mini.tail1234.ts.net:443":true,"jterrazz-mac-mini.tail1234.ts.net:8443":true,"jterrazz-mac-mini.tail1234.ts.net:10000":false}}`

type recorded string

func (r recorded) LookPath(name string) (string, bool) { return "/usr/local/bin/" + name, true }

func (r recorded) Output(_ context.Context, _ local.Cmd) (local.Output, error) {
	return local.Output{Stdout: string(r)}, nil
}

func TestTheFunnelConfigListsThePortsFunnelOpens(t *testing.T) {
	got, err := New(recorded(funnelStatus), noApp{}, "").Funneled(context.Background())
	if err != nil || !slices.Equal(got, []int{443, 8443}) {
		t.Fatalf("Funneled = %v, %v, want [443 8443]", got, err)
	}
}

func TestANodeWithNoServeConfigFunnelsNothing(t *testing.T) {
	got, err := New(recorded(""), noApp{}, "").Funneled(context.Background())
	if err != nil || len(got) != 0 {
		t.Fatalf("Funneled = %v, %v, want nothing", got, err)
	}
}
