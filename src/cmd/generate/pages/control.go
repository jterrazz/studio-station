package pages

import (
	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/engine"
	pageoutcome "github.com/jterrazz/studio-station/src/pages/outcome"
)

func controlPages() []Fixture {
	stopped := engine.StopReport{Daemon: engine.DaemonStopped, Power: engine.PowerEnded, VMs: []engine.Stopped{
		{Name: "linux-1", Lifetime: "temporary", Result: engine.ResultDeleted},
		{Name: "api", Lifetime: "lasting", Result: engine.ResultStopped},
	}}
	stuck := engine.StopReport{Daemon: engine.DaemonKept, VMs: []engine.Stopped{
		{Name: "linux-1", Lifetime: "temporary", Result: engine.ResultDeleted},
		{Name: "api", Lifetime: "lasting", Result: engine.ResultFailed, Detail: catalog.HypervisorTool + " stop exited 1"},
	}}
	started := engine.StartReport{Version: "v3.1.0", PID: 812, Back: []string{"api"}}
	out := states("outcome-stop",
		pending(pageoutcome.Stop(stopped, here())),
		pageoutcome.Stop(engine.StopReport{Daemon: engine.DaemonAlready}, here()),
		pageoutcome.Stop(stuck, here()),
		pageoutcome.Stop(stopped, here()),
	)
	return append(out, states("outcome-start",
		pending(pageoutcome.Start(started, here())),
		pageoutcome.Start(engine.StartReport{Already: true, Version: "v3.1.0", PID: 812}, here()),
		failure("outcome", "start", "the station daemon did not answer in 15s", "tail ~/.local/state/station/logs/daemon.log"),
		pageoutcome.Start(started, here()),
	)...)
}
