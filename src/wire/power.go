package wire

import (
	"context"
	"time"

	"github.com/jterrazz/studio-station/src/awake"

	awakecalls "github.com/jterrazz/studio-station/src/awake/calls"
	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/engine"
	"github.com/jterrazz/studio-station/src/overview"
	"github.com/jterrazz/studio-station/src/page"
	pagepower "github.com/jterrazz/studio-station/src/pages/power"
)

func (m machine) powerPage(v awake.View) any {
	r := awake.Report{Now: m.host.Clock.Now(), Zone: time.Local, Daemon: awake.DaemonRunning, Reading: v.Reading, Session: v.Session, LastEnd: v.LastEnd, Guards: v.Guards}
	return page.Document{Schema: page.Schema, Page: pagepower.Page(r, m.where)}
}

func powerDown(err error) error { return downAs(err, awake.ErrDaemonDown) }

func (m machine) power() *awakecalls.Service {
	c := m.engineAPI()
	return awakecalls.NewService(m.host.Clock, awakecalls.ServicePorts{
		View: func(ctx context.Context) (awake.View, error) {
			var v awake.View
			err := c.do(ctx, engine.CallPower, nil, &v)
			return v, powerDown(err)
		},
		Awake: func(ctx context.Context, r awake.Request) (awake.Awoken, error) {
			var a awake.Awoken
			err := c.do(ctx, engine.CallAwake, engine.AwakeIn{Lid: r.Lid, Battery: r.Battery, For: int(r.For / time.Second), By: r.By}, &a)
			return a, powerDown(err)
		},
		Sleep: func(ctx context.Context) (awake.Slept, error) {
			var s engine.PowerSlept
			err := c.do(ctx, engine.CallSleep, nil, &s)
			return awake.Slept{Ended: s.Ended, After: time.Duration(s.After) * time.Second}, powerDown(err)
		},
		Installed: func() bool {
			_, err := m.host.FS.Stat(catalog.EnginePlist(m.host.Env.Home()))
			return err == nil
		},
	}, awakecalls.ServiceOptions{Power: m.powerPort, Here: m.kind, HereName: m.where.Machine, Zone: time.Local})
}

type statusPower struct{ s *awakecalls.Service }

func (p statusPower) Session(ctx context.Context) (overview.PowerState, bool) {
	if p.s.Check() != nil {
		return overview.PowerState{}, false
	}
	var last awake.Report
	for r := range p.s.Read(ctx) {
		last = r
	}
	if last.Daemon != awake.DaemonRunning {
		return overview.PowerState{}, false
	}
	if last.Session == nil {
		return overview.PowerState{}, true
	}
	return overview.PowerState{Held: true, Lid: last.Session.Lid, Until: last.Session.Until}, true
}
