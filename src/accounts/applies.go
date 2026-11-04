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

var SharedInto machine.Applies = func(p machine.Profile) machine.Verdict {
	// invariant: a share lands only in a machine that keeps accounts: a
	// server keeps its own secrets, and station shares none with it.
	if Applies(p).OK {
		return machine.Yes
	}
	return machine.No(p.Ref.Name + " keeps its own secrets")
}
