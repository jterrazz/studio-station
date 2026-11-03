package app

import (
	"context"
	"time"

	"github.com/spf13/cobra"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/cli/kit"
	powercmd "github.com/jterrazz/studio-station/src/cli/power"
	powerawake "github.com/jterrazz/studio-station/src/cli/power/awake"
	powersleep "github.com/jterrazz/studio-station/src/cli/power/sleep"
	"github.com/jterrazz/studio-station/src/domain/daemon"
	"github.com/jterrazz/studio-station/src/domain/power"
	"github.com/jterrazz/studio-station/src/page"
	pagepower "github.com/jterrazz/studio-station/src/pages/power"
)

// value: the pmset a lid close runs, under STATION_SYSTEM_ROOT in a spec so
// the real one never runs there; launchd's PATH is no guide, so it is absolute.
func (m machine) pmset() string { return m.systemRoot() + "/usr/bin/pmset" }

func (m machine) powerPage(v power.View) any {
	r := power.Report{Now: m.host.Clock.Now(), Zone: time.Local, Daemon: power.DaemonRunning, Reading: v.Reading, Session: v.Session, LastEnd: v.LastEnd, Guards: v.Guards}
	return page.Document{Schema: page.Schema, Page: pagepower.Page(r, m.where)}
}

func powerDown(err error) error { return downAs(err, power.ErrDaemonDown) }

func (m machine) power() *power.Service {
	c := daemon.Dial(m.host, m.paths)
	return power.NewService(m.host, power.Ports{
		View: func(ctx context.Context) (power.View, error) {
			var v power.View
			err := c.Do(ctx, "power", daemon.Request{}, &v)
			return v, powerDown(err)
		},
		Awake: func(ctx context.Context, r power.Request) (power.Awoken, error) {
			var a power.Awoken
			err := c.Do(ctx, "power_awake", daemon.Request{Lid: r.Lid, Battery: r.Battery, For: int(r.For / time.Second), By: r.By}, &a)
			return a, powerDown(err)
		},
		Sleep: func(ctx context.Context) (power.Slept, error) {
			var s daemon.PowerSlept
			err := c.Do(ctx, "power_sleep", daemon.Request{}, &s)
			return power.Slept{Ended: s.Ended, After: time.Duration(s.After) * time.Second}, powerDown(err)
		},
		Installed: func() bool {
			_, err := m.host.FS.Stat(catalog.DaemonPlist(m.host.Env.Home()))
			return err == nil
		},
	}, power.ServiceOptions{Here: m.kind, HereName: m.where.Machine, Zone: time.Local})
}

func powerCommand(env kit.Env, m machine) *cobra.Command {
	s := m.power()
	return powercmd.Command(powercmd.Deps{Power: s, Here: m.where}, env,
		powerawake.Command(powerawake.Deps{Power: s, Here: m.where}, env),
		powersleep.Command(powersleep.Deps{Power: s, Here: m.where}, env),
	)
}
