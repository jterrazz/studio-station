package local

import "github.com/jterrazz/studio-station/src/system"

// value: what an adapter runs a tool and reads a file with, the only door to
// this machine a family's adapter is given.
type (
	Cmd    = system.Cmd
	Output = system.Output
)

var ErrTimeout = system.ErrTimeout
