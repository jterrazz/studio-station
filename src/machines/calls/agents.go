package calls

import (
	"context"
	"path"
	"strings"

	model "github.com/jterrazz/studio-station/src/machines"

	background "github.com/jterrazz/studio-station/src/background/contract"
	"github.com/jterrazz/studio-station/src/machines/contract"
)

const agentPrefix = "com.jterrazz.station.vm."

func (s *Service) LogPath(name string) string { return path.Join(s.o.Logs, name+".log") }

func (s *Service) KnownHostsPath(name string) string { return path.Join(s.o.KnownHosts, name) }

func (s *Service) KeyPath() string { return s.img.KeyPath() }

func (s *Service) VMUser() string {
	cfg, _ := s.prefs()
	return s.vmUser(cfg)
}

func (s *Service) HereName() string {
	cfg, _ := s.prefs()
	return s.here(cfg)
}

func (s *Service) oldAgent(name string) string {
	return path.Join(s.h.Home(), "Library", "LaunchAgents", agentPrefix+name+".plist")
}

func (s *Service) session() background.Domain { return background.Domain{UID: s.h.UID()} }

func (s *Service) oldJob(name string) background.Ref {
	return background.Ref{Domain: s.session(), Label: agentPrefix + name, File: s.oldAgent(name)}
}

// invariant: a VM still under a round-3 per-VM agent has that agent booted out
// and its plist removed by the verb that stops or deletes it, so the setup
// migration's hint can always be followed.
func (s *Service) retireAgent(ctx context.Context, name string) {
	file := s.oldAgent(name)
	if !s.h.Exists(file) {
		return
	}
	_ = s.o.Jobs.Remove(ctx, s.oldJob(name))
}

type AgentMigration struct{ Retired []string }

func (s *Service) MigrateAgents(ctx context.Context) (AgentMigration, error) {
	// invariant: a migration never cuts a running VM: one VM running under an old
	// agent refuses the whole step before any bootout.
	dir := path.Join(s.h.Home(), "Library", "LaunchAgents")
	entries, readErr := s.h.Names(dir)
	if readErr != nil {
		return AgentMigration{}, nil //nolint:nilerr // outside: no LaunchAgents folder holds no agent to migrate.
	}
	var names []string
	for _, n := range entries {
		if strings.HasPrefix(n, agentPrefix) && strings.HasSuffix(n, ".plist") {
			names = append(names, strings.TrimSuffix(strings.TrimPrefix(n, agentPrefix), ".plist"))
		}
	}
	if len(names) == 0 {
		return AgentMigration{}, nil
	}
	var running []string
	for _, n := range names {
		if s.engineState(ctx, n) == contract.DiskRunning {
			running = append(running, n)
		}
	}
	if len(running) > 0 {
		return AgentMigration{}, &model.RunningUnderAgentError{Names: running}
	}
	overrides := s.o.Jobs.Overrides(ctx, s.session())
	records, _ := s.records()
	for _, n := range names {
		_ = s.o.Jobs.Remove(ctx, s.oldJob(n))
		for _, r := range records {
			if r.Name != n || r.Lifetime != model.Permanent {
				continue
			}
			r.Want = model.WantRunning
			if enabled, listed := overrides[agentPrefix+n]; listed && !enabled {
				r.Want = model.WantStopped
			}
			_ = s.writeRecord(r)
		}
	}
	return AgentMigration{Retired: names}, nil
}
