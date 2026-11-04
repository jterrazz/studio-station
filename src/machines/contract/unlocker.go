package contract

import (
	"context"
	"errors"
)

var ErrUnreachable = errors.New("the Mac does not answer")

type RestartRefusedError struct{ Detail string }

func (e *RestartRefusedError) Error() string { return "the Mac refused to restart: " + e.Detail }

type Unlocker interface {
	// invariant: the owner types the password into macOS's own pre-boot prompt
	// on the terminal, never into station; a Mac that does not answer is
	// ErrUnreachable and no session opens.
	Unlock(ctx context.Context, mac string) error
	// invariant: a planned restart unlocks itself once; the Mac dropping the
	// connection to reboot is its success.
	PlannedRestart(ctx context.Context, mac string) error
}
