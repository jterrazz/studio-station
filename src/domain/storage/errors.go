package storage

import (
	"strings"

	"github.com/jterrazz/studio-station/src/catalog"
)

type UnknownToolError struct{ Name string }

func (e *UnknownToolError) Error() string { return "no storage row named \"" + e.Name + "\"" }

type WrongPlaceError struct {
	Name   string
	Places []catalog.MachineKind
	Here   catalog.MachineKind
}

func (e *WrongPlaceError) Error() string {
	places := make([]string, len(e.Places))
	for i, p := range e.Places {
		places[i] = string(p)
	}
	return e.Name + "'s place is " + strings.Join(places, ", ") + "; here is " + e.Here.Article()
}

type NoCleanError struct{ Name string }

func (e *NoCleanError) Error() string { return e.Name + " has no cache station can clean" }

type NoResetError struct{ Name, Hint string }

func (e *NoResetError) Error() string { return e.Name + " has no data station can reset" }

type NoNameError struct{}

func (e *NoNameError) Error() string { return "reset names its tools" }
