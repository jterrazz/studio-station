package status

import (
	"context"
	"encoding/json"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/jterrazz/studio-station/src/catalog"
)

type service struct {
	name     string
	needle   string
	config   string
	model    func(data []byte) string
	port     func(Options) int
	channels bool
}

func serviceTable() []service {
	return []service{
		{
			name: "OpenClaw", needle: "openclaw/dist/index.js", config: ".openclaw/openclaw.json",
			model: openClawModel, port: func(o Options) int { return o.OpenClawPort }, channels: true,
		},
		{
			name: "Hermes", needle: "hermes_cli.main", config: ".hermes/config.yaml",
			model: hermesModel, port: func(o Options) int { return o.HermesPort },
		},
	}
}

func (s *Service) services(ctx context.Context) []Check {
	checks := []Check{}
	if !s.mac() {
		return checks
	}
	ps := s.ask(ctx, cmd("ps", "-axo", "user,pid,ppid,command"), defaultBudget)
	for _, sv := range serviceTable() {
		checks = append(checks, s.serviceChecks(ctx, sv, ps)...)
	}
	return checks
}

func (s *Service) serviceChecks(ctx context.Context, sv service, ps reply) []Check {
	cfgPath := path.Join(s.h.Env.Home(), sv.config)
	data, readErr := s.h.FS.ReadFile(cfgPath)
	process := gatewayLine(ps, sv.needle)
	if process == nil && readErr != nil {
		return nil
	}
	checks := []Check{s.runtime(ctx, sv, process), configCheck(sv, cfgPath, data, readErr)}
	if sv.channels && process != nil {
		checks = append(checks, s.channelChecks(ctx, sv.name)...)
	}
	return checks
}

func (s *Service) runtime(ctx context.Context, sv service, process []string) Check {
	name := sv.name + " runtime"
	if process == nil {
		return Check{Name: name, State: catalog.StateBroken, Value: "not running", Detail: "gateway process not found"}
	}
	port := sv.port(s.o)
	detail := "pid " + process[1] + " · user " + process[0] + " · port " + strconv.Itoa(port)
	if !s.h.Net.Dial(ctx, "127.0.0.1", port, 750*time.Millisecond) {
		return Check{Name: name, State: catalog.StateDegraded, Value: "running", Detail: detail + " not listening"}
	}
	return Check{Name: name, State: catalog.StateOK, Value: "running", Detail: detail}
}

func configCheck(sv service, cfgPath string, data []byte, readErr error) Check {
	name := sv.name + " config"
	if readErr != nil {
		return Check{Name: name, State: catalog.StateBroken, Value: "missing", Detail: cfgPath}
	}
	model := sv.model(data)
	if model == "" {
		model = "configured"
	}
	if model == invalidConfig {
		return Check{Name: name, State: catalog.StateBroken, Value: "invalid", Detail: cfgPath}
	}
	return Check{Name: name, State: catalog.StateOK, Value: model, Detail: cfgPath}
}

const invalidConfig = "\x00invalid"

// outside: `ps -axo user,pid,ppid,command` prints one process per line; the gateway
// is the one whose command holds the service's needle and the word `gateway`.
func gatewayLine(ps reply, needle string) []string {
	for _, line := range strings.Split(ps.text(), "\n") {
		if strings.Contains(line, needle) && strings.Contains(line, "gateway") {
			if fields := strings.Fields(line); len(fields) >= 2 {
				return fields
			}
		}
	}
	return nil
}

func openClawModel(data []byte) string {
	var cfg map[string]any
	if json.Unmarshal(data, &cfg) != nil {
		return invalidConfig
	}
	if m := jsonPath(cfg, "models", "default"); m != "" {
		return m
	}
	return jsonPath(cfg, "agents", "default", "model")
}

func jsonPath(root map[string]any, keys ...string) string {
	var current any = root
	for _, key := range keys {
		m, ok := current.(map[string]any)
		if !ok {
			return ""
		}
		current = m[key]
	}
	text, _ := current.(string)
	return text
}

// value: a two-state line scanner reads `model:` then `  default: <value>` without a
// yaml dependency; the file is small and its layout stable.
func hermesModel(data []byte) string {
	inModel := false
	for _, raw := range strings.Split(string(data), "\n") {
		line := strings.TrimRight(raw, " \t\r")
		stripped := strings.TrimSpace(line)
		if stripped == "" || strings.HasPrefix(stripped, "#") {
			continue
		}
		if line[0] != ' ' && line[0] != '\t' {
			inModel = strings.HasPrefix(stripped, "model:")
			continue
		}
		if rest, ok := strings.CutPrefix(stripped, "default:"); ok && inModel {
			return strings.Trim(strings.TrimSpace(rest), "\"'")
		}
	}
	return ""
}

func channelNames() []string { return []string{"Slack", "Telegram", "BlueBubbles"} }

func (s *Service) channelChecks(ctx context.Context, owner string) []Check {
	r := s.ask(ctx, cmd("openclaw", "channels", "status"), repoBudget)
	if !r.ok() {
		return []Check{{Name: owner + " channels", State: catalog.StateUnknown}}
	}
	checks := make([]Check, 0, len(channelNames()))
	for _, channel := range channelNames() {
		checks = append(checks, channelCheck(owner+" "+channel, channel, r.text()))
	}
	return checks
}

func channelCheck(name, channel, status string) Check {
	for _, line := range strings.Split(status, "\n") {
		if !strings.Contains(line, channel+" ") && !strings.Contains(line, channel+":") {
			continue
		}
		line = strings.TrimSpace(line)
		unhealthy := strings.Contains(line, "health:") && !strings.Contains(line, "health:healthy")
		if !strings.Contains(line, "connected") || unhealthy {
			return Check{Name: name, State: catalog.StateDegraded, Value: "check", Detail: line}
		}
		return Check{Name: name, State: catalog.StateOK, Value: "connected", Detail: line}
	}
	return Check{Name: name, State: catalog.StateAbsent, Value: "not reported"}
}
