package wire

import (
	"context"
	"time"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/domain/power"
	"github.com/jterrazz/studio-station/src/engine"
	"github.com/jterrazz/studio-station/src/engine/process"
	"github.com/jterrazz/studio-station/src/overview"
	"github.com/jterrazz/studio-station/src/page"
	pagekeepawake "github.com/jterrazz/studio-station/src/pages/keepawake"
)

func (m machine) keepAwakePage(v power.View) any {
	r := power.Report{Now: m.sys.Clock.Now(), Zone: time.Local, Engine: power.DaemonRunning, Reading: v.Reading, Session: v.Session, LastEnd: v.LastEnd, Guards: v.Guards}
	return page.Document{Schema: page.Schema, Page: pagekeepawake.Page(r, m.where)}
}

func awakeDown(err error) error { return downAs(err, power.ErrDaemonDown) }

func (m machine) awakeService() *power.Service {
	c := m.engineAPI()
	return power.NewService(m.sys, power.Ports{
		View: func(ctx context.Context) (power.View, error) {
			var v power.View
			err := c.do(ctx, process.AwakeView, nil, &v)
			return v, awakeDown(err)
		},
		Awake: func(ctx context.Context, r power.Request) (power.Awoken, error) {
			var a power.Awoken
			err := c.do(ctx, process.KeepAwake, engine.AwakeIn{Lid: r.Lid, Battery: r.Battery, For: int(r.For / time.Second), By: r.By}, &a)
			return a, awakeDown(err)
		},
		Sleep: func(ctx context.Context) (power.Slept, error) {
			var s engine.AwakeOff
			err := c.do(ctx, process.AwakeOff, nil, &s)
			return power.Slept{Ended: s.Ended, After: time.Duration(s.After) * time.Second}, awakeDown(err)
		},
		Installed: func() bool {
			_, err := m.sys.FS.Stat(catalog.EnginePlist(m.sys.Env.Home()))
			return err == nil
		},
	}, power.ServiceOptions{Power: m.powerPort, Here: m.kind, HereName: m.where.Machine, Zone: time.Local})
}

type statusAwake struct{ s *power.Service }

func (p statusAwake) Session(ctx context.Context) (overview.AwakeState, bool) {
	if p.s.Check() != nil {
		return overview.AwakeState{}, false
	}
	var last power.Report
	for r := range p.s.Read(ctx) {
		last = r
	}
	if last.Engine != power.DaemonRunning {
		return overview.AwakeState{}, false
	}
	if last.Session == nil {
		return overview.AwakeState{}, true
	}
	return overview.AwakeState{Held: true, Lid: last.Session.Lid, Until: last.Session.Until}, true
}
