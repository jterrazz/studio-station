package status

import (
	"context"
	"path"
	"sort"
	"strings"

	"github.com/jterrazz/studio-station/src/catalog"
)

// value: only the estate's own agents: macOS and third-party apps ship dozens more.
func daemonPrefixes() []string {
	return []string{"ai.jterrazz.", "com.jterrazz.", "ai.openclaw.", "ai.hermes."}
}

func (s *Service) daemons(ctx context.Context) []Check {
	checks := []Check{}
	if !s.mac() {
		return checks
	}
	entries, err := s.h.FS.ReadDir(path.Join(s.h.Env.Home(), "Library", "LaunchAgents"))
	if err != nil {
		return checks
	}
	var labels []string
	for _, e := range entries {
		label, ok := strings.CutSuffix(e.Name(), ".plist")
		if !ok {
			continue
		}
		for _, prefix := range daemonPrefixes() {
			if strings.HasPrefix(label, prefix) {
				labels = append(labels, label)
				break
			}
		}
	}
	if len(labels) == 0 {
		return checks
	}
	sort.Strings(labels)
	r := s.ask(ctx, cmd("launchctl", "list"), defaultBudget)
	for _, label := range labels {
		checks = append(checks, daemonState(label, r))
	}
	return checks
}

// outside: `launchctl list` prints `<pid or -><TAB><last exit><TAB><label>` per loaded job.
func daemonState(label string, r reply) Check {
	if !r.ok() {
		return Check{Name: label, State: catalog.StateUnknown}
	}
	for _, line := range strings.Split(r.text(), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 || fields[2] != label {
			continue
		}
		pid, exit := fields[0], fields[1]
		switch {
		case pid != "-":
			return Check{Name: label, State: catalog.StateOK, Value: "running · pid " + pid}
		case exit != "0" && exit != "-":
			return Check{Name: label, State: catalog.StateBroken, Value: "idle · last exit " + exit}
		}
		return Check{Name: label, State: catalog.StateOK, Value: "idle"}
	}
	return Check{Name: label, State: catalog.StateAbsent, Value: "not loaded"}
}
