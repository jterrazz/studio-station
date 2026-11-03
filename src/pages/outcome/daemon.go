package outcome

import (
	"strconv"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/domain/daemon"
	"github.com/jterrazz/studio-station/src/page"
	"github.com/jterrazz/studio-station/src/pages"
)

func vmRow(v daemon.Stopped) page.Row {
	switch v.Result {
	case daemon.ResultDeleted:
		return pages.Done(v.Name, catalog.StateOK, "deleted ("+v.Lifetime+")")
	case daemon.ResultStopped:
		return pages.Done(v.Name, catalog.StateOK, "stopped ("+v.Lifetime+", disk kept)")
	}
	r := pages.Done(v.Name, catalog.StateBroken, "")
	r.Detail = page.Texts("did not stop", v.Detail)
	return r
}

func daemonRow(state, stuck string) page.Row {
	switch state {
	case daemon.DaemonStopped:
		r := pages.Done("daemon", catalog.StateOK, "")
		r.Detail = page.Texts("stopped", daemon.Label+" booted out")
		return r
	case daemon.DaemonAlready:
		return pages.Done("daemon", catalog.StateOK, "already stopped")
	}
	return pages.Done("daemon", catalog.StateAbsent, "kept running: "+stuck+" did not stop")
}

func Stop(r daemon.StopReport, here page.Context) page.Page {
	rows := make([]page.Row, 0, len(r.VMs)+1)
	stuck := ""
	for _, v := range r.VMs {
		rows = append(rows, vmRow(v))
		if v.Result == daemon.ResultFailed && stuck == "" {
			stuck = v.Name
		}
	}
	switch r.Power {
	case daemon.PowerEnded:
		p := pages.Done("power", catalog.StateOK, "")
		p.Detail = page.Texts("ended", "the clamshell reset to normal")
		rows = append(rows, p)
	case daemon.PowerReset:
		rows = append(rows, pages.Done("power", catalog.StateOK, "reset"))
	}
	if r.Daemon != "" {
		rows = append(rows, daemonRow(r.Daemon, stuck))
	}
	p := pages.Outcome("stop", here, nil, rows, "VMs to stop")
	if r.Daemon == daemon.DaemonStopped || r.Daemon == daemon.DaemonAlready {
		p.Hints = []page.Hint{*page.Cmd("station start")}
	}
	return p
}

func StopConfirm(w daemon.Work, host string) page.Confirm {
	lines := make([]page.Value, 0, len(w.VMs)+1)
	for _, v := range w.VMs {
		if v.Result == daemon.ResultDeleted {
			lines = append(lines, page.Text(v.Name+" ("+v.Lifetime+") will be deleted"))
		} else {
			lines = append(lines, page.Text(v.Name+" ("+v.Lifetime+") will stop; its disk is kept"))
		}
	}
	lines = append(lines, page.Text("the daemon stays stopped until station start or the next login"))
	return page.Confirm{Question: "Stop everything on " + host + "?", Lines: lines, Danger: true, Yes: "Stop"}
}

func Start(r daemon.StartReport, here page.Context) page.Page {
	pid := "pid " + strconv.Itoa(r.PID)
	d := pages.Done("daemon", catalog.StateOK, "")
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
	return pages.Outcome("start", here, nil, rows, "daemon to start")
}
