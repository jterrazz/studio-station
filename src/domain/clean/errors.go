package clean

import (
	"strings"

	"github.com/jterrazz/studio-station/src/catalog"
)

type UnknownCleanableError struct{ Name string }

func (e *UnknownCleanableError) Error() string { return "unknown cleanable \"" + e.Name + "\"" }

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
	return e.Name + "'s place is " + strings.Join(places, ", ") + "; here is " + string(e.Here)
}
