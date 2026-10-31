package status

import (
	"context"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/jterrazz/studio-station/src/catalog"
)

func (s *Service) environment(ctx context.Context) Environment {
	var (
		env Environment
		wg  sync.WaitGroup
	)
	run := func(fn func()) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			fn()
		}()
	}
	run(func() { env.Network = s.probes(ctx, s.networkProbes()) })
	if s.mac() {
		run(func() { env.Security = s.probes(ctx, securityProbes()) })
		run(func() { env.Updates = s.probes(ctx, updateProbes()) })
		run(func() {
			env.Ports = listeningPorts(s.ask(ctx, cmd("lsof", "-iTCP", "-sTCP:LISTEN", "-P", "-n", "-Fpcn"), defaultBudget))
		})
	}
	wg.Wait()
	return env
}

func (s *Service) networkProbes() []probe {
	if !s.mac() {
		return []probe{
			{name: "local ip", cmd: cmd("hostname", "-I"), interpret: firstField},
			{name: "public ip", cmd: cmd("curl", "-s", "--max-time", "2", "-4", "ifconfig.me"), budget: defaultBudget + defaultBudget/2, interpret: firstField},
			{name: "dns", cmd: cmd("cat", "/etc/resolv.conf"), interpret: resolvConf},
		}
	}
	return []probe{
		{name: "local ip", cmd: cmd("ipconfig", "getifaddr", "en0"), interpret: firstField},
		{name: "public ip", cmd: cmd("curl", "-s", "--max-time", "2", "-4", "ifconfig.me"), budget: defaultBudget + defaultBudget/2, interpret: firstField},
		{name: "vpn", cmd: cmd("scutil", "--nc", "list"), interpret: vpns},
		{name: "dns", cmd: cmd("scutil", "--dns"), interpret: scutilDNS},
	}
}

func securityProbes() []probe {
	return []probe{
		{name: "filevault", cmd: cmd("fdesetup", "status"), interpret: onWhen("FileVault is On")},
		{name: "firewall", cmd: cmd("socketfilterfw", "--getglobalstate"), interpret: onWhen("enabled")},
		{name: "sip", cmd: cmd("csrutil", "status"), interpret: onWhen("enabled")},
		{name: "gatekeeper", cmd: cmd("spctl", "--status"), interpret: onWhen("enabled")},
	}
}

func updateProbes() []probe {
	return []probe{
		{name: "macos", cmd: cmd("softwareupdate", "-l"), budget: updateBudget, interpret: macUpdates},
		{name: "brew", cmd: cmd("brew", "outdated", "--quiet"), budget: updateBudget, interpret: brewOutdated},
	}
}

func firstField(name string, r reply) Check {
	fields := strings.Fields(r.text())
	if !r.ok() || len(fields) == 0 {
		return Check{Name: name, State: catalog.StateAbsent}
	}
	return Check{Name: name, State: catalog.StateOK, Value: fields[0]}
}

// outside: scutil lists a connection as `* (Connected) <id> VPN (...) "Name" [...]`.
func vpns(name string, r reply) Check {
	if !r.ok() {
		return Check{Name: name, State: catalog.StateUnknown}
	}
	var names []string
	for _, line := range strings.Split(r.text(), "\n") {
		if !strings.Contains(line, "(Connected)") {
			continue
		}
		label := "connected"
		if end := strings.LastIndex(line, `"`); end > 0 {
			if start := strings.LastIndex(line[:end], `"`); start >= 0 {
				label = line[start+1 : end]
			}
		}
		names = append(names, label)
	}
	if len(names) == 0 {
		return Check{Name: name, State: catalog.StateAbsent, Value: "off"}
	}
	return Check{Name: name, State: catalog.StateOK, Value: strings.Join(names, ", ")}
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

// invariant: a tool that is missing or fails answers unknown, never off: the row
// must not claim a protection is down on a machine that could not look.
func onWhen(needle string) func(string, reply) Check {
	return func(name string, r reply) Check {
		if !r.ok() {
			return Check{Name: name, State: catalog.StateUnknown}
		}
		if strings.Contains(r.all(), needle) {
			return Check{Name: name, State: catalog.StateOK, Value: "on"}
		}
		return Check{Name: name, State: catalog.StateAbsent, Value: "off"}
	}
}

// outside: softwareupdate prints its verdict on stderr, and lists each update as `* Label: ...`.
func macUpdates(name string, r reply) Check {
	if !r.ok() {
		return Check{Name: name, State: catalog.StateUnknown}
	}
	pending := 0
	for _, line := range strings.Split(r.all(), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "* ") {
			pending++
		}
	}
	if pending == 0 {
		return Check{Name: name, State: catalog.StateOK, Value: "up to date"}
	}
	return Check{Name: name, State: catalog.StateDegraded, Value: strconv.Itoa(pending) + " pending", Detail: "softwareupdate -l"}
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

// outside: lsof -F prints one field per line, p<pid>, c<command>, n<address>.
func listeningPorts(r reply) []Port {
	ports := []Port{}
	if !r.ok() {
		return ports
	}
	var pid int
	var command string
	seen := map[string]bool{}
	for _, line := range strings.Split(r.text(), "\n") {
		if line == "" {
			continue
		}
		field := line[1:]
		switch line[0] {
		case 'p':
			pid = int(parseInt(field))
		case 'c':
			command = field
		case 'n':
			i := strings.LastIndex(field, ":")
			if command == "" || systemPortProcess(command) || i < 0 {
				continue
			}
			port := field[i+1:]
			if key := port + "/" + command; !seen[key] {
				seen[key] = true
				ports = append(ports, Port{PID: pid, Name: command, Port: port})
			}
		}
	}
	sort.SliceStable(ports, func(i, j int) bool { return parseInt(ports[i].Port) < parseInt(ports[j].Port) })
	return ports
}
