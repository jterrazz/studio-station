package accounts

import (
	"slices"

	"github.com/jterrazz/studio-station/src/core/machine"
	"github.com/jterrazz/studio-station/src/core/section"
)

var Applies machine.Applies = func(p machine.Profile) machine.Verdict {
	// invariant: accounts are every machine's whose section table holds them.
	if slices.Contains(section.For(p.Kind), section.Accounts) {
		return machine.Yes
	}
	return machine.No("accounts are not for this machine")
}

var SharedFrom machine.Applies = func(p machine.Profile) machine.Verdict {
	// invariant: a share is a Mac's: its accounts are copied into its own VMs.
	if p.Kind == machine.Mac {
		return machine.Yes
	}
	return machine.No("accounts are shared from a Mac")
}
