package overview

import (
	"time"

	"github.com/jterrazz/studio-station/src/accounts"
	"github.com/jterrazz/studio-station/src/apps"
	"github.com/jterrazz/studio-station/src/awake"
	"github.com/jterrazz/studio-station/src/background"
	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/core/machine"
	"github.com/jterrazz/studio-station/src/core/section"
	"github.com/jterrazz/studio-station/src/disk"
	"github.com/jterrazz/studio-station/src/ports"
	"github.com/jterrazz/studio-station/src/settings"
)

func counted(parts []Part) []Part {
	// invariant: a line names only the states it has; one with none says so
	// by its first count, zero.
	out := []Part{}
	for i, p := range parts {
		if !p.Counted || p.N > 0 || i == 0 {
			out = append(out, p)
		}
	}
	return out
}

func AppsRead(reports []apps.Report, m machine.Ref) Read {
	n := apps.Counts(reports)
	updates := "updates"
	if n[apps.UpdateAvailable] == 1 {
		updates = "update"
	}
	return Read{Line: Line{Section: section.Apps, Pending: n[apps.Pending] > 0, Parts: counted([]Part{
		Count(catalog.StateOK, n[apps.OK], "ok", string(apps.OK)),
		Count(catalog.StateDegraded, n[apps.UpdateAvailable], updates, string(apps.UpdateAvailable)),
		Count(catalog.StateBroken, n[apps.Failed], "failed", string(apps.Failed)),
		Count(catalog.StateUnknown, n[apps.NotManaged], "not managed", string(apps.NotManaged)),
		Count(catalog.StateUnknown, n[apps.NotKnown], "not known", string(apps.NotKnown)),
		Count(catalog.StateAbsent, n[apps.NotInstalled], "not installed", string(apps.NotInstalled)),
	})}, Findings: apps.Findings(reports, m)}
}

func SettingsRead(rows []settings.Row, m machine.Ref) Read {
	var total, applied, changed, off int
	pending := false
	for _, r := range rows {
		if r.Group != settings.SetByStation || r.State == settings.StateNotForThisMachine {
			continue
		}
		total++
		switch r.State {
		case settings.StateApplied:
			applied++
		case settings.StateChangedSince:
			changed++
		case settings.StateOffHere:
			off++
		case settings.StateReading:
			pending = true
		case settings.StateNotApplied, settings.StateNotForThisMachine:
		}
	}
	parts := append([]Part{{Word: "set by station"}, {State: catalog.StateOK, N: applied, Of: total, Key: string(settings.StateApplied), Counted: true}},
		counted([]Part{{}, Count(catalog.StateDegraded, changed, "changed since", string(settings.StateChangedSince)), Count(catalog.StateAbsent, off, "off here", string(settings.StateOffHere))})[1:]...)
	return Read{Line: Line{Section: section.Settings, Pending: pending, Parts: parts}, Findings: settings.Findings(rows, m)}
}

func DiskRead(r disk.Reading, m machine.Ref) Read {
	misplaced := len(r.Of(disk.Misplaced))
	state := catalog.StateOK
	if misplaced > 0 {
		state = catalog.StateDegraded
	}
	parts := []Part{{State: state, Bytes: r.Free, Word: "free"}, {Bytes: disk.Sum(r.Of(disk.CanRebuild)), Word: "can rebuild"}}
	if misplaced > 0 {
		parts = append(parts, Count(catalog.StateDegraded, misplaced, "misplaced", string(disk.Misplaced)))
	}
	return Read{Line: Line{Section: section.Disk, Pending: r.Pending, Parts: parts}, Findings: disk.Findings(r, m)}
}

func PortsRead(l ports.Listing) Read {
	// invariant: every port station shares is private (a VM's, on the
	// tailnet); a public one is a declared port, none until they exist.
	private := 0
	for _, h := range l.Fleet.Hosts {
		private += len(h.Services)
	}
	return Read{Line: Line{Section: section.Ports, Parts: []Part{{Word: "Public", Key: "public", Counted: true}, {Word: "Private", N: private, Key: "private", Counted: true}}}}
}

func AccountsRead(r accounts.Report, m machine.Ref) Read {
	ok := 0
	var broken []Part
	for _, mm := range r.Machines {
		if !mm.Here {
			continue
		}
		for _, a := range mm.Accounts {
			switch a.State {
			case catalog.StateOK:
				ok++
			case catalog.StateBroken:
				broken = append(broken, Part{State: catalog.StateBroken, Word: a.Provider})
			case catalog.StatePending:
				return Pending(section.Accounts)
			case catalog.StateDegraded, catalog.StateAbsent, catalog.StateUnknown:
			}
		}
	}
	return Read{Line: Line{Section: section.Accounts, Parts: append([]Part{Count(catalog.StateOK, ok, "", "signed-in")}, broken...)}, Findings: accounts.Findings(r, m)}
}

func BackgroundRead(tasks []background.Task, m machine.Ref) Read {
	var failing, orphans int
	for _, t := range tasks {
		switch {
		case t.Run == background.Failing:
			failing++
		case t.Group == background.GroupOrphan:
			orphans++
		}
	}
	return Read{Line: Line{Section: section.Background, Parts: counted([]Part{
		Count(catalog.StateOK, len(tasks)-failing-orphans, "", "ok"),
		Count(catalog.StateBroken, failing, "failing", string(background.Failing)),
		Count(catalog.StateDegraded, orphans, "orphans", string(background.GroupOrphan)),
	})}, Findings: background.Findings(tasks, m)}
}

func AwakeRead(r awake.Report, m machine.Ref) Read {
	// invariant: Awake is the engine's: with the engine stopped its line
	// says so rather than read off.
	if r.Daemon != "" && r.Daemon != awake.EngineRunning && r.Daemon != awake.EngineReading {
		return Unread(section.Awake, "needs the engine")
	}
	var parts []Part
	switch r.Mode() {
	case awake.NeverSleeps:
		parts = []Part{{State: catalog.StateOK, Word: "never sleeps"}}
	case awake.AwakeUntil:
		s := r.Session
		zone := r.Zone
		if zone == nil {
			zone = time.UTC
		}
		parts = []Part{{State: catalog.StateOK, Word: "until " + s.Until.In(zone).Format("15:04")}}
		if s.Lid {
			parts = append(parts, Part{Word: "lid closed"})
		}
	case awake.StoppedByGuard:
		parts = []Part{{State: catalog.StateDegraded, Word: "stopped"}, {Word: r.LastEnd.Detail}}
	case awake.SleepsNormally:
		parts = []Part{{State: catalog.StateAbsent, Word: "off"}}
	}
	return Read{Line: Line{Section: section.Awake, Parts: parts}, Findings: awake.Findings(r, m)}
}

func SSHRead(m machine.Ref) Read {
	// invariant: every Mac's ssh config holds a block for each VM by its
	// name, hopping through its Mac: the one door, from any Mac, is its name.
	return Read{Line: Line{Section: section.SSH, Parts: []Part{{Word: m.Name}}}}
}
