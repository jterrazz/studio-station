package catalog

import "time"

type ServiceDefaults struct {
	RangeLow, RangeHigh, MaxPerVM, MinPort int
	Ignored                                []int
	HTTPS                                  string
	Idle, Grace                            time.Duration
}

var ServicePolicy = ServiceDefaults{
	RangeLow: 40000, RangeHigh: 40999, MaxPerVM: 20, MinPort: 1024,
	Ignored: []int{22}, HTTPS: "port", Idle: 7 * 24 * time.Hour, Grace: 5 * time.Second,
}

type ProfileEnv struct{ Name, Value string }

type Profile struct {
	Name string
	Port int
	HTTP bool
	Env  []ProfileEnv
	Note string
}

var Profiles = []Profile{
	{Name: "expo", Port: 8081, HTTP: true, Env: []ProfileEnv{{Name: "EXPO_PACKAGER_PROXY_URL", Value: "{url}"}}},
	{Name: "vite", Port: 5173, HTTP: true, Env: []ProfileEnv{{Name: "__VITE_ADDITIONAL_SERVER_ALLOWED_HOSTS", Value: "{host}"}}},
}

const TailnetProduct = "Tailscale" // value: the product that carries a share, as the owner reads its name.

const ServeRow = "Tailscale Serve" // value: the row that says whether the tailnet serves this host's VMs.

var SystemListeners = []string{"rapportd", "ControlCenter", "IPNExtension", "mDNSResponder", "launchd", "systemd"} // value: the processes whose listening port is the system's own: noise in a developer's list.

// value: why a reserved port of a VM that does not run is not shared.
func StoppedDetail(vm string) string { return "not shared while " + vm + " is stopped" }

const ServeOffDetail = "Serve is off on Tailscale: turn on HTTPS certificates in the Tailscale admin console"

var PublicHostEnv = ProfileEnv{Name: "STATION_PUBLIC_HOST", Value: "{host}"}

// value: the two Tailscale app variants, measured 2026-11-01: the Standalone
// app's pkg installs the first CLI path, the App Store app only its bundle,
// whose executable answers when run directly (a symlink to it crashes).
const (
	TailscaleApp           = "/Applications/Tailscale.app"
	TailscaleReceipt       = TailscaleApp + "/Contents/_MASReceipt"
	TailscaleBundleCLI     = TailscaleApp + "/Contents/MacOS/Tailscale"
	TailscaleStandaloneCLI = "/usr/local/bin/tailscale"
)

var TailscaleCLIs = []string{TailscaleStandaloneCLI, TailscaleBundleCLI}

const ReachableOutside = "reachable outside " + TailnetProduct // value: the words of a port anything outside the tailnet reaches.

type KnownPort struct {
	Port    int
	Process string
	What    string
	// value: where the owner turns it off, a System Settings path.
	Off string
}

const airPlayOff = "System Settings > General > AirDrop & Handoff > AirPlay Receiver"

var KnownPorts = []KnownPort{ // value: the ports whose name the owner knows better than its process's; one with no process is known on any.
	{Port: 5000, Process: "ControlCenter", What: "AirPlay Receiver", Off: airPlayOff},
	{Port: 7000, Process: "ControlCenter", What: "AirPlay Receiver", Off: airPlayOff},
	{Port: 22, What: "SSH"},
	{Port: 5900, What: "Screen Sharing"},
	{Port: 6443, What: "Kubernetes API"},
}

func KnownPortOf(port int, process string) (KnownPort, bool) {
	for _, k := range KnownPorts {
		if k.Port == port && (k.Process == "" || k.Process == process) {
			return k, true
		}
	}
	return KnownPort{}, false
}
