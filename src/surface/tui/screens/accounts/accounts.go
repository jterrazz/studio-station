package accountsscreen

import (
	"strings"

	"github.com/jterrazz/studio-station/src/page"
	pageaccounts "github.com/jterrazz/studio-station/src/pages/accounts"

	"github.com/jterrazz/studio-station/src/surface/tui/screens/gesture"
)

// value: an account row is `<machine>/<account>`, under its machine's group.
func parts(r page.Row) (machine, account string) {
	machine, account, _ = strings.Cut(r.ID, "/")
	return machine, account
}

func Gesture(p page.Page, a page.Action, rows []page.Row) (gesture.Gesture, bool) {
	// invariant: an account of this Mac is shared with one of its VMs, picked;
	// an account a VM holds is taken back from it.
	if len(rows) != 1 {
		return gesture.Gesture{}, false
	}
	machine, account := parts(rows[0])
	if account == "" {
		return gesture.Gesture{}, false
	}
	thisMac := machine == p.Context.Machine
	switch a.Key {
	case pageaccounts.KeyShare:
		vms := others(p)
		if !thisMac || len(vms) == 0 {
			return gesture.Gesture{}, false
		}
		return gesture.Gesture{Pick: &gesture.Pick{Title: "Share " + account + " with", Options: vms, Then: func(vm string) gesture.Gesture {
			return gesture.Gesture{Steps: []gesture.Step{gesture.V("Share", "Sharing", "Shared").Step(account+" with "+vm, []string{"accounts", "share", account, "--with", vm})}}
		}}}, true
	case pageaccounts.KeyTakeBack:
		if thisMac {
			return gesture.Gesture{}, false
		}
		return gesture.Gesture{Steps: []gesture.Step{{Words: []string{"accounts", "take-back", account, "--from", machine}, Ask: "Take " + account + " back from " + machine + "?", Doing: "Taking " + account + " back from " + machine, Done: "Took " + account + " back from " + machine}}}, true
	}
	return gesture.Gesture{}, false
}

// value: the VMs the page lists, every machine group but this Mac's.
func others(p page.Page) []string {
	var out []string
	for _, sec := range p.Sections {
		for _, it := range sec.Items {
			if it.Group != nil && it.Group.Head.ID != p.Context.Machine && it.Group.Head.ID != "" {
				out = append(out, it.Group.Head.ID)
			}
		}
	}
	return out
}
