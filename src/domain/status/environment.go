package status

import (
	"context"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/ports"
)

func (s *Service) networkProbes() []probe {
	if !s.mac() {
		return []probe{
			{name: "local ip", cmd: cmd("hostname", "-I"), interpret: firstField},
			{name: "dns", cmd: cmd("cat", "/etc/resolv.conf"), interpret: resolvConf},
		}
	}
	return []probe{
		{name: "local ip", cmd: cmd("ipconfig", "getifaddr", "en0"), interpret: firstField},
		{name: "dns", cmd: cmd("scutil", "--dns"), interpret: scutilDNS},
	}
}

func (s *Service) securityProbes() []probe {
	return []probe{
		s.factProbe("filevault", catalog.FactFileVault, defaultBudget),
		s.factProbe("firewall", catalog.FactFirewall, defaultBudget),
		s.factProbe("sip", catalog.FactSIP, defaultBudget),
		s.factProbe("gatekeeper", catalog.FactGatekeeper, defaultBudget),
	}
}

// invariant: a fact past its budget answers late, whatever the machine says.
func (s *Service) factProbe(name string, f catalog.Fact, budget time.Duration) probe {
	return probe{name: name, budget: budget, read: func(ctx context.Context, budget time.Duration) Check {
		ctx, cancel := context.WithTimeout(ctx, budget)
		defer cancel()
		fact, err := s.deps.Inspector.Read(ctx, f)
		if err != nil {
			return late(name, budget)
		}
		return Check{Name: name, State: fact.State, Value: fact.Value, Detail: fact.Detail}
	}}
}

func firstField(name string, r reply) Check {
	fields := strings.Fields(r.text())
	if !r.ok() || len(fields) == 0 {
		return Check{Name: name, State: catalog.StateAbsent}
	}
	return Check{Name: name, State: catalog.StateOK, Value: fields[0]}
}

// outside: `scutil --dns` lists `nameserver[0] : 9.9.9.9`; loopback and IPv6 resolvers
// are the local stub, not a provider.
func scutilDNS(name string, r reply) Check {
	var servers []string
	for _, line := range strings.Split(r.text(), "\n") {
		_, server, ok := strings.Cut(line, "nameserver[")
		if !ok {
			continue
		}
		_, server, ok = strings.Cut(server, "] : ")
		servers = appendResolver(servers, strings.TrimSpace(server), ok)
	}
	return resolvers(name, r, servers)
}

func resolvConf(name string, r reply) Check {
	var servers []string
	for _, line := range strings.Split(r.text(), "\n") {
		if rest, ok := strings.CutPrefix(strings.TrimSpace(line), "nameserver"); ok {
			servers = appendResolver(servers, strings.TrimSpace(rest), true)
		}
	}
	return resolvers(name, r, servers)
}

func appendResolver(servers []string, server string, ok bool) []string {
	switch {
	case !ok, server == "", server == "127.0.0.1", strings.Contains(server, ":"), len(servers) >= 2:
		return servers
	}
	for _, have := range servers {
		if have == server {
			return servers
		}
	}
	return append(servers, server)
}

func resolvers(name string, r reply, servers []string) Check {
	if !r.ok() || len(servers) == 0 {
		return Check{Name: name, State: catalog.StateAbsent}
	}
	return Check{Name: name, State: catalog.StateOK, Value: strings.Join(servers, ", ")}
}

func brewOutdated(name string, r reply) Check {
	switch {
	case !r.found:
		return Check{Name: name, State: catalog.StateAbsent, Value: "not installed"}
	case !r.ok():
		return Check{Name: name, State: catalog.StateUnknown}
	}
	if n := len(strings.Fields(r.text())); n > 0 {
		return Check{Name: name, State: catalog.StateDegraded, Value: strconv.Itoa(n) + " outdated", Detail: "station tools upgrade --via brew"}
	}
	return Check{Name: name, State: catalog.StateOK, Value: "all current"}
}

// value: processes whose listening port is the system's own (sharing, the control
// center, the mDNS resolver, the tailnet extension) are noise in a developer's list.
func systemPortProcess(name string) bool {
	switch name {
	case "rapportd", "ControlCenter", "IPNExtension", "mDNSResponder", "launchd", "systemd":
		return true
	}
	return false
}

// invariant: one row per port and process, the system's own listeners left
// out, in port order.
func listeningPorts(ls []ports.Listener) []Port {
	out := []Port{}
	seen := map[string]bool{}
	for _, l := range ls {
		if l.Process == "" || systemPortProcess(l.Process) {
			continue
		}
		port := strconv.Itoa(l.Port)
		if key := port + "/" + l.Process; !seen[key] {
			seen[key] = true
			out = append(out, Port{PID: l.PID, Name: l.Process, Port: port})
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return parseInt(out[i].Port) < parseInt(out[j].Port) })
	return out
}

func (s *Service) listening(ctx context.Context) []Port {
	if s.deps.Listeners == nil {
		return []Port{}
	}
	ctx, cancel := context.WithTimeout(ctx, defaultBudget)
	defer cancel()
	ls, err := s.deps.Listeners.Read(ctx)
	if err != nil || ctx.Err() != nil {
		return []Port{}
	}
	return listeningPorts(ls)
}

// invariant: Tailscale Serve is the way every VM service leaves the host, so a
// host whose tailnet is connected says whether it is on, and the admin step
// when it is not.
func serveGateway(on bool) Check {
	if on {
		return Check{Name: "Tailscale Serve", State: catalog.StateOK, Value: "on"}
	}
	return Check{Name: "Tailscale Serve", State: catalog.StateDegraded, Value: "off", Detail: catalog.ServeOffDetail}
}
