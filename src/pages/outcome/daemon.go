package outcome

import (
	"strconv"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/engine"
	"github.com/jterrazz/studio-station/src/page"
	"github.com/jterrazz/studio-station/src/pages"
)

func vmRow(v engine.Stopped) page.Row {
	switch v.Result {
	case engine.ResultDeleted:
		return pages.Done(v.Name, catalog.StateOK, "deleted ("+v.Lifetime+")")
	case engine.ResultStopped:
		return pages.Done(v.Name, catalog.StateOK, "stopped ("+v.Lifetime+", disk kept)")
	}
	r := pages.Done(v.Name, catalog.StateBroken, "")
	r.Detail = page.Texts("did not stop", v.Detail)
	return r
}

func daemonRow(state, stuck string) page.Row {
	switch state {
	case engine.DaemonStopped:
		r := pages.Done("engine", catalog.StateOK, "")
		r.Detail = page.Texts("stopped", engine.Label+" booted out")
		return r
	case engine.DaemonAlready:
		return pages.Done("engine", catalog.StateOK, "already stopped")
	}
	return pages.Done("engine", catalog.StateAbsent, "kept running: "+stuck+" did not stop")
}

func Stop(r engine.StopReport, here page.Context) page.Page {
	rows := make([]page.Row, 0, len(r.VMs)+1)
	stuck := ""
	for _, v := range r.VMs {
		rows = append(rows, vmRow(v))
		if v.Result == engine.ResultFailed && stuck == "" {
			stuck = v.Name
		}
	}
	switch r.Power {
	case engine.PowerEnded:
		p := pages.Done("power", catalog.StateOK, "")
		p.Detail = page.Texts("ended", "the clamshell reset to normal")
		rows = append(rows, p)
	case engine.PowerReset:
		rows = append(rows, pages.Done("power", catalog.StateOK, "reset"))
	}
	if r.Daemon != "" {
		rows = append(rows, daemonRow(r.Daemon, stuck))
	}
	p := pages.Outcome("stop", here, nil, rows, "VMs to stop")
	if r.Daemon == engine.DaemonStopped || r.Daemon == engine.DaemonAlready {
		p.Hints = []page.Hint{*page.Cmd("station engine start")}
	}
	return p
}

func StopConfirm(w engine.Work, host string) page.Confirm {
	lines := make([]page.Value, 0, len(w.VMs)+1)
	for _, v := range w.VMs {
		if v.Result == engine.ResultDeleted {
			lines = append(lines, page.Text(v.Name+" ("+v.Lifetime+") will be deleted"))
		} else {
			lines = append(lines, page.Text(v.Name+" ("+v.Lifetime+") will stop; its disk is kept"))
		}
	}
	lines = append(lines, page.Text("the engine stays stopped until station engine start or the next login"))
	return page.Confirm{Question: "Stop everything on " + host + "?", Lines: lines, Danger: true, Yes: "Stop"}
}

func Start(r engine.StartReport, here page.Context) page.Page {
	pid := "pid " + strconv.Itoa(r.PID)
	d := pages.Done("engine", catalog.StateOK, "")
	if r.Already {
		d.Detail = page.Texts("running", pid)
	} else {
		d.Detail = page.Texts("started", r.Version, pid)
	}
	rows := make([]page.Row, 0, 1+len(r.Back))
	rows = append(rows, d)
	for _, vm := range r.Back {
		rows = append(rows, pages.Done(vm, catalog.StateOK, "started (lasting, wish running)"))
	}
	return pages.Outcome("start", here, nil, rows, "engine to start")
}
