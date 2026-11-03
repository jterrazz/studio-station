package pages

import (
	"github.com/jterrazz/studio-station/src/domain/daemon"
	pageoutcome "github.com/jterrazz/studio-station/src/pages/outcome"
)

func controlPages() []Fixture {
	stopped := daemon.StopReport{Daemon: daemon.DaemonStopped, Power: daemon.PowerEnded, VMs: []daemon.Stopped{
		{Name: "linux-1", Lifetime: "temporary", Result: daemon.ResultDeleted},
		{Name: "api", Lifetime: "lasting", Result: daemon.ResultStopped},
	}}
	stuck := daemon.StopReport{Daemon: daemon.DaemonKept, VMs: []daemon.Stopped{
		{Name: "linux-1", Lifetime: "temporary", Result: daemon.ResultDeleted},
		{Name: "api", Lifetime: "lasting", Result: daemon.ResultFailed, Detail: "tart stop exited 1"},
	}}
	started := daemon.StartReport{Version: "v3.1.0", PID: 812, Back: []string{"api"}}
	out := states("outcome-stop",
		pending(pageoutcome.Stop(stopped, here())),
		pageoutcome.Stop(daemon.StopReport{Daemon: daemon.DaemonAlready}, here()),
		pageoutcome.Stop(stuck, here()),
		pageoutcome.Stop(stopped, here()),
	)
	return append(out, states("outcome-start",
		pending(pageoutcome.Start(started, here())),
		pageoutcome.Start(daemon.StartReport{Already: true, Version: "v3.1.0", PID: 812}, here()),
		failure("outcome", "start", "the station daemon did not answer in 15s", "tail ~/.local/state/station/logs/daemon.log"),
		pageoutcome.Start(started, here()),
	)...)
}
