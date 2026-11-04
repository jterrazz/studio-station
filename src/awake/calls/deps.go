package calls

import (
	"context"
	"time"

	"github.com/jterrazz/studio-station/src/awake"
	"github.com/jterrazz/studio-station/src/core/call"
)

type Deps struct {
	Power Power
}

type Power interface {
	// value: refuses a machine Power does not run on, in its own words.
	Check() error
	// value: refuses a session shorter or longer than Power holds.
	CheckFor(term time.Duration) error
	Read(ctx context.Context) <-chan awake.Report
	Awake(ctx context.Context, r awake.Request) (awake.Awoken, error)
	Sleep(ctx context.Context) (awake.Slept, error)
}

func All(d Deps) []call.Handler {
	return []call.Handler{call.BindWatch(Read, d), call.Bind(Keep, d), call.Bind(Sleep, d)}
}
