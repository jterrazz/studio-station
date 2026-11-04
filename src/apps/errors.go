package apps

import "github.com/jterrazz/studio-station/src/catalog"

type UnknownError struct{ Name string }

func (e *UnknownError) Error() string { return "no app named \"" + e.Name + "\"" }

type NeedsError struct{ App, Needs string }

func (e *NeedsError) Error() string { return e.App + " needs " + e.Needs + ", which is not installed" }

type ElsewhereError struct {
	App    string
	Places []catalog.MachineKind
	// value: a running VM where the app lives, named for the hint; empty
	// when none runs.
	VM string
}

func (e *ElsewhereError) Error() string { return e.App + " lives on " + LivesOn(e.Places) }

type NotInstalledError struct{ App string }

func (e *NotInstalledError) Error() string { return e.App + " is not installed" }

func LivesOn(places []catalog.MachineKind) string {
	// value: where an app lives, in the owner's words: a Mac is This Mac and
	// any other Mac, a VM a workspace.
	switch livesOn(places) {
	case "every-machine":
		return "every machine"
	case "mac-only":
		return "Macs only"
	case "vms-only":
		return "VMs only"
	case "linux-vms-only":
		return "Linux VMs only"
	case "macos-vms-only":
		return "macOS VMs only"
	}
	return "no machine"
}

func LivesOnToken(places []catalog.MachineKind) string {
	// value: where an app lives, as station.apps.list/v1's lives_on spells it.
	return livesOn(places)
}

func livesOn(places []catalog.MachineKind) string {
	var mac, linux, macVM bool
	for _, p := range places {
		switch p {
		case catalog.Host:
			mac = true
		case catalog.Linux:
			linux = true
		case catalog.MacOS:
			macVM = true
		}
	}
	switch {
	case mac && linux:
		return "every-machine"
	case mac:
		return "mac-only"
	case linux && macVM:
		return "vms-only"
	case linux:
		return "linux-vms-only"
	case macVM:
		return "macos-vms-only"
	}
	return "no-machine"
}
