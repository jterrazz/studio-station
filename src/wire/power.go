package wire

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/domain/power"
	"github.com/jterrazz/studio-station/src/engine"
	"github.com/jterrazz/studio-station/src/engine/process"
	"github.com/jterrazz/studio-station/src/overview"
	"github.com/jterrazz/studio-station/src/page"
	"github.com/jterrazz/studio-station/src/pages"
	pagepower "github.com/jterrazz/studio-station/src/pages/power"
)

func (m machine) powerPage(v power.View) any {
	r := power.Report{Now: m.host.Clock.Now(), Zone: time.Local, Daemon: power.DaemonRunning, Reading: v.Reading, Session: v.Session, LastEnd: v.LastEnd, Guards: v.Guards}
	return page.Document{Schema: page.Schema, Page: pagepower.Page(r, m.where)}
}

func powerDown(err error) error { return downAs(err, power.ErrDaemonDown) }

func (m machine) power() *power.Service {
	c := m.engineAPI()
	return power.NewService(m.host, power.Ports{
		View: func(ctx context.Context) (power.View, error) {
			var v power.View
			err := c.do(ctx, process.Power, nil, &v)
			return v, powerDown(err)
		},
		Awake: func(ctx context.Context, r power.Request) (power.Awoken, error) {
			var a power.Awoken
			err := c.do(ctx, process.Awake, engine.AwakeIn{Lid: r.Lid, Battery: r.Battery, For: int(r.For / time.Second), By: r.By}, &a)
			return a, powerDown(err)
		},
		Sleep: func(ctx context.Context) (power.Slept, error) {
			var s engine.PowerSlept
			err := c.do(ctx, process.Sleep, nil, &s)
			return power.Slept{Ended: s.Ended, After: time.Duration(s.After) * time.Second}, powerDown(err)
		},
		Installed: func() bool {
			_, err := m.host.FS.Stat(catalog.EnginePlist(m.host.Env.Home()))
			return err == nil
		},
	}, power.ServiceOptions{Power: m.powerPort, Here: m.kind, HereName: m.where.Machine, Zone: time.Local})
}

type tuiPower struct {
	*power.Service
	m        machine
	describe func(error) (message, hint string)
}

func (t tuiPower) read(ctx context.Context, out chan<- page.Page) power.Report {
	var last power.Report
	for r := range t.Read(ctx) {
		last = r
		select {
		case out <- pagepower.Page(r, t.m.where):
		case <-ctx.Done():
		}
	}
	return last
}

func (t tuiPower) Pages(ctx context.Context) <-chan page.Page {
	out := make(chan page.Page)
	go func() {
		defer close(out)
		if err := t.Check(); err != nil {
			message, hint := t.describe(err)
			out <- pages.Failure("power", "power", t.m.where, message, hint)
			return
		}
		if t.read(ctx, out).Daemon != power.DaemonRunning {
			return
		}
		stream, err := t.m.engineAPI().c.Watch(ctx, process.Page, process.PagesIn{Pages: []string{engine.PagePower}})
		if err != nil {
			return
		}
		for raw := range stream {
			var ev engine.Event
			var d page.Document
			if json.Unmarshal(raw, &ev) != nil || ev.Event != "page" || json.Unmarshal(ev.Page, &d) != nil {
				continue
			}
			select {
			case out <- d.Page:
			case <-ctx.Done():
				return
			}
		}
		if ctx.Err() == nil {
			t.read(ctx, out)
		}
	}()
	return out
}

type statusPower struct{ s *power.Service }

func (p statusPower) Session(ctx context.Context) (overview.PowerState, bool) {
	if p.s.Check() != nil {
		return overview.PowerState{}, false
	}
	var last power.Report
	for r := range p.s.Read(ctx) {
		last = r
	}
	if last.Daemon != power.DaemonRunning {
		return overview.PowerState{}, false
	}
	if last.Session == nil {
		return overview.PowerState{}, true
	}
	return overview.PowerState{Held: true, Lid: last.Session.Lid, Until: last.Session.Until}, true
}
