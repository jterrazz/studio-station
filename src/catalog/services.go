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

const ServeRow = "Tailscale Serve" // value: the row that says whether the tailnet serves this host's VMs.

var SystemListeners = []string{"rapportd", "ControlCenter", "IPNExtension", "mDNSResponder", "launchd", "systemd"} // value: the processes whose listening port is the system's own: noise in a developer's list.

const ServeOffDetail = "Tailscale HTTPS is off: enable HTTPS certificates in the Tailscale admin console"

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
