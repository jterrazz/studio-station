package wire

import (
	"context"

	"github.com/spf13/cobra"

	activitylinux "github.com/jterrazz/studio-station/src/activity/adapter/linux"
	activitymacos "github.com/jterrazz/studio-station/src/activity/adapter/macos"
	activitycontract "github.com/jterrazz/studio-station/src/activity/contract"
	"github.com/jterrazz/studio-station/src/apps/adapter/brew"
	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/domain/config"
	"github.com/jterrazz/studio-station/src/domain/machines"
	"github.com/jterrazz/studio-station/src/domain/setup"
	"github.com/jterrazz/studio-station/src/domain/storage"
	"github.com/jterrazz/studio-station/src/overview"
	overviewcalls "github.com/jterrazz/studio-station/src/overview/calls"
	"github.com/jterrazz/studio-station/src/ports/adapter/lsof"
	portscontract "github.com/jterrazz/studio-station/src/ports/contract"
	settingscli "github.com/jterrazz/studio-station/src/settings/adapter/cli"
	settingslinux "github.com/jterrazz/studio-station/src/settings/adapter/linux"
	settingsmacos "github.com/jterrazz/studio-station/src/settings/adapter/macos"
	settingscontract "github.com/jterrazz/studio-station/src/settings/contract"
	"github.com/jterrazz/studio-station/src/surface/cli/kit"
	overviewcmd "github.com/jterrazz/studio-station/src/surface/cli/overview"
)

func overviewCommand(env kit.Env, m machine) *cobra.Command {
	return overviewcmd.Command(m.overviewDeps(false), env)
}

// value: the one Overview `overview --on <vm>` and `machines <vm>` both
// print (MA-06): a VM is read from All machines.
func (m machine) overviewDeps(status bool) overviewcmd.Deps {
	return overviewcmd.Deps{Here: m.where, Status: status, Fleet: func(ctx context.Context) (machines.Fleet, error) {
		return m.machines(false).Fleet(ctx, false)
	}}
}

func statusCommand(env kit.Env, m machine) *cobra.Command {
	return kit.Alias("status", overviewcmd.Command(m.overviewDeps(true), env), "overview")
}

func statusDeps(env kit.Env, m machine) overviewcalls.Sections {
	h := m.host
	d := overviewcalls.Sections{
		Pool:  m.pool,
		Clock: h.Clock,
		Here: overviewcalls.Here{
			Kind: m.kind, Hostname: h.Env.Hostname(), OS: h.Env.GOOS(), Arch: h.Env.GOARCH(), User: h.Env.User(), Shell: h.Env.Shell(),
			Home: h.Env.Home(), Root: m.systemRoot(), Version: Version(), Commit: Commit(),
			Agent: func() bool {
				_, err := h.FS.Stat(catalog.EnginePlist(h.Env.Home()))
				return err == nil
			},
		},
		Name:       m.stores.name(),
		Identity:   m.stores.identity(),
		Owner:      settingscli.New(h.Run, h.FS, settingscli.Options{Home: h.Env.Home(), Pool: m.pool}),
		Tailnet:    m.tailnetStatus(),
		Power:      statusPower{s: m.power()},
		Tools:      toolsService(env, m),
		Config:     configRead{configService(m)},
		Setup:      setupRead{setupService(env, m)},
		Disk:       diskRead{m.storage(env)},
		Daemon:     m.hostDaemon(),
		Hypervisor: m.hypervisor(),
	}
	var facts settingscontract.Inspector
	var meter activitycontract.Meter
	var listening portscontract.Listeners = lsof.New(h.Run)
	if m.kind == catalog.Linux {
		facts, meter = settingslinux.New(h.Run), activitylinux.New(h.Run, h.Clock)
	} else {
		facts, meter = settingsmacos.New(h.Run, h.FS, h.Clock, settingsmacos.Options{Root: m.systemRoot()}), activitymacos.New(h.Run, h.Clock)
	}
	if m.world != nil {
		facts, meter, listening = m.world.inspector(), m.world.meter(), m.world.listeners()
	}
	if m.kind != catalog.Linux {
		// value: the status page reads listeners, jobs and app updates on a Mac only.
		d.Listeners, d.Jobs, d.Updates = listening, m.jobs(overviewcalls.ReadBudget), brew.New(h.Run, nil)
	}
	d.Facts, d.Meter = facts, meter
	return d
}

type configRead struct{ *config.Service }

func (configRead) Placed(r config.Report) (string, catalog.State) { return r.Item.Name, r.State }

type setupRead struct{ s *setup.Service }

func (r setupRead) Check(ctx context.Context) ([]overview.Step, error) {
	report, err := r.s.Check(ctx)
	if err != nil {
		return nil, err
	}
	steps := make([]overview.Step, 0, len(report.Steps))
	for _, s := range report.Steps {
		steps = append(steps, overview.Step{Name: s.Name, State: s.State, Detail: s.Detail})
	}
	return steps, nil
}

type diskRead struct{ s *storage.Service }

func (r diskRead) Watch(ctx context.Context) <-chan overviewcalls.DiskReading[storage.StrayReport] {
	out := make(chan overviewcalls.DiskReading[storage.StrayReport])
	go func() {
		defer close(out)
		for report := range r.s.Watch(ctx) {
			select {
			case out <- diskOf(report):
			case <-ctx.Done():
				return
			}
		}
	}()
	return out
}

const levelInstalled catalog.Level = "installed"

// invariant: one row per storage row and level it holds bytes at, then its
// installed size, then `other`, the caches no row claims.
func diskOf(report storage.Report) overviewcalls.DiskReading[storage.StrayReport] {
	rows := []overview.Disk{}
	for _, r := range report.Rows {
		for _, level := range []catalog.Level{catalog.LevelCache, catalog.LevelData} {
			if b := r.Bytes[level]; b > 0 {
				rows = append(rows, overview.Disk{Name: r.Entry.Name, Level: level, Bytes: b})
			}
		}
		if r.Install > 0 {
			rows = append(rows, overview.Disk{Name: r.Entry.Name, Level: levelInstalled, Bytes: r.Install})
		}
	}
	if report.Other > 0 {
		rows = append(rows, overview.Disk{Name: "other", Level: catalog.LevelCache, Bytes: report.Other})
	}
	return overviewcalls.DiskReading[storage.StrayReport]{Rows: rows, Strays: append(make([]storage.StrayReport, 0, len(report.Strays)), report.Strays...)}
}
