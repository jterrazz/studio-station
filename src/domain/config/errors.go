package config

import (
	"strings"

	"github.com/jterrazz/studio-station/src/catalog"
)

type UnknownItemError struct{ Name string }

func (e *UnknownItemError) Error() string { return "unknown config item \"" + e.Name + "\"" }

type NeedsRootError struct{ Item, Verb string }

func (e *NeedsRootError) Error() string { return e.Item + " needs root" }

type WrongPlaceError struct {
	Item   string
	Places []catalog.MachineKind
	Here   catalog.MachineKind
}

func (e *WrongPlaceError) Error() string {
	places := make([]string, len(e.Places))
	for i, p := range e.Places {
		places[i] = string(p)
	}
	return e.Item + "'s place is " + strings.Join(places, ", ") + "; here is " + e.Here.Article()
}

type NotReversibleError struct{ Item string }

func (e *NotReversibleError) Error() string { return e.Item + " cannot be uninstalled by station" }

type NeedsTerminalError struct{ Item string }

func (e *NeedsTerminalError) Error() string { return e.Item + " needs a terminal" }

type StudioMissingError struct{ Item string }

func (e *StudioMissingError) Error() string {
	return e.Item + " is kept by the studio CLI, which is not on PATH"
}

type unhandledError struct{ Item string }

func (e *unhandledError) Error() string { return "no handler for config item \"" + e.Item + "\"" }
