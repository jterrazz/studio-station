package macos

import (
	"context"
	"strings"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/settings"
)

// outside: on Apple Silicon SleepOnPowerButton and hibernatemode are firmware-controlled
// and absent from `pmset -g custom`, so they are neither set nor checked.
func (m *Mac) observePower(ctx context.Context, want []settings.KeyValue) settings.Observation {
	return judged(m.powerValues(ctx, want), want, "restarts after a power cut", "partly applied")
}

func (m *Mac) applyPower(ctx context.Context, it catalog.ConfigItem, _ map[string]string) (settings.Change, error) {
	policy := settings.Sets(it)
	change := settings.Change{Before: m.powerValues(ctx, policy)}
	args := make([]string, 0, 1+2*len(policy))
	args = append(args, "-a")
	for _, kv := range policy {
		args = append(args, kv.Key, kv.Value)
	}
	if _, err := m.must(ctx, "pmset", args...); err != nil {
		return change, err
	}
	for _, kv := range policy {
		change.Steps = append(change.Steps, done(kv.Key+"="+kv.Value, ""))
	}
	change.After = m.powerValues(ctx, policy)
	return change, nil
}

func (m *Mac) undoPower(ctx context.Context, it catalog.ConfigItem) (settings.Change, error) {
	policy := settings.Sets(it)
	change := settings.Change{Before: m.powerValues(ctx, policy)}
	args := make([]string, 0, 1+2*len(policy))
	args = append(args, "-a")
	for _, kv := range policy {
		args = append(args, kv.Key, "0")
	}
	if _, err := m.must(ctx, "pmset", args...); err != nil {
		return change, err
	}
	for _, kv := range policy {
		change.Steps = append(change.Steps, done(kv.Key+"=0", ""))
	}
	change.After = m.powerValues(ctx, policy)
	return change, nil
}

// value: the policy's keys as pmset reads them now; a key it does not
// print is empty.
func (m *Mac) powerValues(ctx context.Context, policy []settings.KeyValue) []settings.KeyValue {
	out, ok := m.probe(ctx, "pmset", "-g", "custom")
	if !ok {
		return nil
	}
	current := map[string]string{}
	for _, line := range strings.Split(out, "\n") {
		if fields := strings.Fields(line); len(fields) >= 2 {
			current[fields[0]] = fields[1]
		}
	}
	values := make([]settings.KeyValue, 0, len(policy))
	for _, kv := range policy {
		values = append(values, settings.KeyValue{Key: kv.Key, Value: current[kv.Key]})
	}
	return values
}
