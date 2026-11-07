package contract

import "context"

type Session interface {
	// value: the terminal session of a VM the agents run in, the one the
	// owner attaches to.
	//
	// invariant: Open runs argv in folder inside the session, opening the
	// session when none runs and a new window of it when one does.
	Open(ctx context.Context, folder string, argv []string) error
	// invariant: Send types text into the session's current pane, then
	// Enter; the text travels on stdin, never on a command line.
	Send(ctx context.Context, text string) error
}
