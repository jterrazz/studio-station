package config

import (
	"context"
	"strings"

	"github.com/jterrazz/studio-station/src/catalog"
)

// outside: on Apple Silicon SleepOnPowerButton and hibernatemode are firmware-controlled
// and absent from `pmset -g custom`, so they are neither set nor checked.
func powerPolicy() [][2]string {
	return [][2]string{
		{"autorestart", "1"},
		{"sleep", "0"},
		{"displaysleep", "5"},
		{"disksleep", "0"},
		{"powernap", "0"},
		{"womp", "1"},
	}
}

func (s *Service) installPower(ctx context.Context) (Change, error) {
	if err := s.requireDarwin(); err != nil {
		return Change{}, err
	}
	change := Change{Before: s.powerFacts(ctx)}
	args := []string{"-a"}
	for _, kv := range powerPolicy() {
		args = append(args, kv[0], kv[1])
	}
	if _, err := s.must(ctx, "pmset", args...); err != nil {
		return change, err
	}
	for _, kv := range powerPolicy() {
		change.Steps = append(change.Steps, okStep(kv[0]+"="+kv[1], ""))
	}
	change.After = s.powerFacts(ctx)
	return change, nil
}

func (s *Service) uninstallPower(ctx context.Context) (Change, error) {
	if err := s.requireDarwin(); err != nil {
		return Change{}, err
	}
	change := Change{Before: s.powerFacts(ctx)}
	if _, err := s.must(ctx, "pmset", "-a", "-resetdefaults"); err != nil {
		return change, err
	}
	change.Steps = []Step{okStep("pmset", "reset to the macOS defaults")}
	change.After = s.powerFacts(ctx)
	return change, nil
}

func (s *Service) checkPower(ctx context.Context) (state catalog.State, detail string) {
	out, ok := s.probe(ctx, "pmset", "-g", "custom")
	if !ok {
		return catalog.StateAbsent, ""
	}
	current := pmsetValues(out)
	matched := 0
	for _, kv := range powerPolicy() {
		if current[kv[0]] == kv[1] {
			matched++
		}
	}
	switch matched {
	case len(powerPolicy()):
		return catalog.StateOK, "always-on policy applied"
	case 0:
		return catalog.StateAbsent, ""
	}
	return catalog.StateDegraded, "policy partly applied"
}

func (s *Service) powerFacts(ctx context.Context) []Fact {
	out, ok := s.probe(ctx, "pmset", "-g", "custom")
	if !ok {
		return []Fact{{Key: "pmset", Value: "unreadable"}}
	}
	current := pmsetValues(out)
	facts := make([]Fact, 0, len(powerPolicy()))
	for _, kv := range powerPolicy() {
		value := current[kv[0]]
		if value == "" {
			value = "-"
		}
		facts = append(facts, Fact{Key: kv[0], Value: value})
	}
	return facts
}

func pmsetValues(out string) map[string]string {
	values := map[string]string{}
	for _, line := range strings.Split(out, "\n") {
		if fields := strings.Fields(line); len(fields) >= 2 {
			values[fields[0]] = fields[1]
		}
	}
	return values
}
