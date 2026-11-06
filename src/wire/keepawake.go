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
	pagekeepawake "github.com/jterrazz/studio-station/src/pages/keepawake"
)

func (m machine) keepAwakePage(v awake.View) any {
	r := awake.Report{Now: m.sys.Clock.Now(), Zone: time.Local, Engine: awake.DaemonRunning, Reading: v.Reading, Session: v.Session, LastEnd: v.LastEnd, Guards: v.Guards}
	return page.Document{Schema: page.Schema, Page: pagekeepawake.Page(r, m.where)}
}

func awakeDown(err error) error { return downAs(err, awake.ErrDaemonDown) }

func (m machine) awakeService() *awakecalls.Service {
	c := m.engineAPI()
	return awakecalls.NewService(m.sys.Clock, awakecalls.ServicePorts{
		View: func(ctx context.Context) (awake.View, error) {
			var v awake.View
			err := c.do(ctx, engine.CallAwakeView, nil, &v)
			return v, awakeDown(err)
		},
		Awake: func(ctx context.Context, r awake.Request) (awake.Awoken, error) {
			var a awake.Awoken
			err := c.do(ctx, engine.CallKeepAwake, engine.AwakeIn{Lid: r.Lid, Battery: r.Battery, For: int(r.For / time.Second), By: r.By}, &a)
			return a, awakeDown(err)
		},
		Sleep: func(ctx context.Context) (awake.Slept, error) {
			var s engine.AwakeOff
			err := c.do(ctx, engine.CallAwakeOff, nil, &s)
			return awake.Slept{Ended: s.Ended, After: time.Duration(s.After) * time.Second}, awakeDown(err)
		},
		Installed: func() bool {
			_, err := m.sys.FS.Stat(catalog.EnginePlist(m.sys.Env.Home()))
			return err == nil
		},
	}, awakecalls.ServiceOptions{Power: m.powerPort, Here: m.kind, HereName: m.where.Machine, Zone: time.Local})
}

type statusAwake struct{ s *awakecalls.Service }

func (p statusAwake) Session(ctx context.Context) (overview.AwakeState, bool) {
	if p.s.Check() != nil {
		return overview.AwakeState{}, false
	}
	var last awake.Report
	for r := range p.s.Read(ctx) {
		last = r
	}
	if last.Engine != awake.DaemonRunning {
		return overview.AwakeState{}, false
	}
	if last.Session == nil {
		return overview.AwakeState{}, true
	}
	return overview.AwakeState{Held: true, Lid: last.Session.Lid, Until: last.Session.Until}, true
}
