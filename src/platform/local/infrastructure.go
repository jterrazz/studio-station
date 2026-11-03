package local

import "github.com/jterrazz/studio-station/src/system"

// invariant: the infrastructure an adapter is handed, named here so that an
// adapter reaches it through platform alone.
type (
	Cmd    = system.Cmd
	Output = system.Output
	Runner = system.Runner
	FS     = system.FS
)

var (
	ErrNotFound = system.ErrNotFound
	ErrTimeout  = system.ErrTimeout
)
