package calls

import (
	"context"

	"github.com/jterrazz/studio-station/src/core/call"
	"github.com/jterrazz/studio-station/src/core/machine"
	"github.com/jterrazz/studio-station/src/disk"
)

type Deps struct {
	Storage Storage
	Kind    machine.Kind
}

type Storage interface {
	Watch(ctx context.Context) <-chan disk.Report
	Clean(ctx context.Context, names []string) ([]disk.Outcome, error)
	CheckReset(names []string) ([]disk.Entry, error)
	Reset(ctx context.Context, names []string) ([]disk.Outcome, error)
}

func All(d Deps) []call.Handler {
	return []call.Handler{call.BindWatch(List, d), call.Bind(Clean, d), call.Bind(Reset, d)}
}
