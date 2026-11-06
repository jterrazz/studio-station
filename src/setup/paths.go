package setup

import (
	"github.com/jterrazz/studio-station/src/machines"
)

type Roots struct{ Config, Data, State, Cache string }

type Paths struct {
	Config, Data, State, Cache, Bin string
	Roots                           Roots
}

type Host = machines.MacRecord

type V1Preferences struct {
	// value: station's preferences as a v1 config.json converts to them, one
	// field per group.
	Here      string
	Hosts     map[string]Host
	VMUser    string
	UserEmail string
	UserName  string
}
