package tools

import (
	"strings"

	"github.com/jterrazz/studio-station/src/catalog"
)

type UnknownToolError struct{ Name string }

func (e *UnknownToolError) Error() string { return "unknown tool \"" + e.Name + "\"" }

type MissingDependencyError struct{ Tool, Requires string }

func (e *MissingDependencyError) Error() string {
	return e.Tool + " requires " + e.Requires + ", which is not installed"
}

type NotReversibleError struct{ Tool string }

func (e *NotReversibleError) Error() string { return e.Tool + " cannot be uninstalled by station" }

type ManualInstallError struct{ Tool, Hint string }

func (e *ManualInstallError) Error() string { return e.Tool + " is installed by hand" }

type NoUpgradePathError struct{ Tool string }

func (e *NoUpgradePathError) Error() string { return e.Tool + " has no upgrade path" }

type WrongPlaceError struct {
	Tool   string
	Places []catalog.MachineKind
	Here   catalog.MachineKind
	VM     string
}

func (e *WrongPlaceError) Error() string {
	places := make([]string, len(e.Places))
	for i, p := range e.Places {
		places[i] = string(p)
	}
	return e.Tool + "'s place is " + strings.Join(places, ", ") + "; here is " + article(e.Here)
}

func article(k catalog.MachineKind) string {
	if k == catalog.Host {
		return "a host"
	}
	return string(k)
}
