package apps

import (
	"strconv"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/core/finding"
	"github.com/jterrazz/studio-station/src/core/machine"
	"github.com/jterrazz/studio-station/src/core/section"
)

func Findings(reports []Report, m machine.Ref) []finding.Finding {
	// invariant: Apps' Needs-you lines: each failed app broken; the updates
	// waiting need a look, one line per manager, in the order its first app
	// is listed, fixed by updating that manager's apps.
	var out []finding.Finding
	waiting := map[catalog.Manager]int{}
	var order []catalog.Manager
	for _, r := range reports {
		switch StateOf(r) {
		case Failed:
			out = append(out, finding.Finding{Machine: m, Section: section.Apps, Severity: finding.Broken, Subject: r.Tool.Name,
				Text: r.Tool.Name + " failed", Fix: finding.Fix{Call: "apps.update", Args: map[string]string{"name": r.Tool.Name}}})
		case UpdateAvailable:
			manager := ManagerOf(r.Method)
			if waiting[manager] == 0 {
				order = append(order, manager)
			}
			waiting[manager]++
		case NotInstalled, NotManaged, OK, NotKnown, Pending:
		}
	}
	for _, manager := range order {
		text := strconv.Itoa(waiting[manager]) + " " + string(manager) + " updates waiting"
		if waiting[manager] == 1 {
			text = "1 " + string(manager) + " update waiting"
		}
		out = append(out, finding.Finding{Machine: m, Section: section.Apps, Severity: finding.NeedsALook, Subject: string(manager),
			Text: text, Fix: finding.Fix{Call: "apps.update", Args: map[string]string{"method": string(manager)}}})
	}
	return out
}

func ManagerOf(method catalog.Method) catalog.Manager {
	// value: the manager whose update brings a method's apps; a method no
	// manager lists is its own.
	for _, m := range catalog.Managers {
		for _, held := range m.Methods {
			if held == method {
				return m.Manager
			}
		}
	}
	return catalog.Manager(method)
}
