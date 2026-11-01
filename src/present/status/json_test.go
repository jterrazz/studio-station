package status_test

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/domain/config"
	"github.com/jterrazz/studio-station/src/domain/setup"
	"github.com/jterrazz/studio-station/src/domain/status"
	"github.com/jterrazz/studio-station/src/domain/tailnet"
	"github.com/jterrazz/studio-station/src/domain/tools"
	presentstatus "github.com/jterrazz/studio-station/src/present/status"
)

var update = flag.Bool("update", false, "rewrite the golden from the presenter")

func fixed() status.Snapshot {
	node := catalog.Tool{Name: "node", Category: catalog.CategoryRuntimes, Installs: catalog.Anywhere(catalog.Mise("node", "lts")), Provides: []string{"node", "npm", "npx"}}
	return status.Snapshot{
		GeneratedAt: time.Date(2026, 10, 31, 14, 0, 0, 0, time.UTC),
		Station:     status.Station{Version: "v3.0.0", Commit: "abc1234", Daemon: status.Check{Name: "daemon", State: catalog.StateOK, Value: "running v3.0.0"}},
		Machine: status.Machine{
			Name: "macbook", Registered: true, Kind: catalog.Host, Hostname: "macbook",
			OS: "Darwin 27.0.0", Arch: "arm64", User: "jterrazz", Shell: "zsh",
		},
		System: status.System{
			Uptime: "3d 4h", Battery: "92% charging", MemoryTotal: 34359738368,
			Sample: status.Sample{
				CPU:    []status.Process{{PID: 1, Name: "x", Value: 12.5}},
				Memory: []status.Process{{PID: 1, Name: "x", Value: 1048576}},
			},
		},
		Environment: status.Environment{
			Network:  []status.Check{{Name: "local ip", State: catalog.StateOK, Value: "192.168.1.2"}},
			Ports:    []status.Port{{PID: 1, Name: "node", Port: "3000"}},
			Security: []status.Check{{Name: "filevault", State: catalog.StateOK, Value: "on"}},
			Updates:  []status.Check{{Name: "brew", State: catalog.StateDegraded, Value: "3 outdated"}},
		},
		Workspace: status.Workspace{
			Repositories: []status.Repository{{Path: "jterrazz/x", Branch: "main"}},
			Dependencies: []status.Dependency{{Path: "jterrazz/x", Manager: "pnpm", Outdated: 2}},
			Docker:       status.Docker{State: catalog.StateOK},
			Disk:         []status.Disk{{Name: "homebrew", Bytes: 123}},
		},
		Tools: []tools.Report{{Tool: node, Method: catalog.MethodMise, State: catalog.StateOK, Version: "24.11.0", Managed: true}},
		Config: []config.Report{{
			Item:  catalog.ConfigItem{Name: "hushlogin", Category: catalog.CategoryTerminal, Places: []catalog.MachineKind{catalog.Host, catalog.Linux, catalog.MacOS}},
			State: catalog.StateOK,
		}},
		Setup: status.Setup{State: catalog.StateOK, Steps: []setup.Step{
			{Name: "name", State: catalog.StateOK, Detail: "macbook"},
			{Name: "zshrc", State: catalog.StateOK},
			{Name: "layout", State: catalog.StateOK},
		}},
		Identity: []status.Check{{Name: "git-email", State: catalog.StateOK, Value: "admin@jterrazz.com"}},
		Daemons:  []status.Check{{Name: "ai.jterrazz.x", State: catalog.StateOK, Value: "running · pid 12"}},
		Gateways: []status.Check{{Name: "OpenClaw runtime", State: catalog.StateBroken, Value: "not running", Detail: "gateway process not found"}},
		Tailnet: tailnet.Status{
			State: catalog.StateOK, Connected: true, BackendState: "Running",
			Name: "macbook", DNSName: "macbook.tail1234.ts.net", IP: "100.64.0.1",
			Variant: "app-store", CLI: "/Applications/Tailscale.app/Contents/MacOS/Tailscale",
		},
	}
}

func TestTheStatusEncodingIsPinned(t *testing.T) {
	body, err := json.MarshalIndent(presentstatus.Snapshot(fixed(), "macbook · host").JSON, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	got := string(body) + "\n"
	golden := filepath.Join("testdata", "status.v1.golden.json")
	if *update {
		if err = os.WriteFile(golden, []byte(got), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatal(err)
	}
	if got != string(want) {
		t.Errorf("station.status/v1 encoding changed; a field is only ADDED within v1 (-update rewrites it).\ngot:\n%s\nwant:\n%s", got, want)
	}
}
