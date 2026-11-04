package calls

import (
	"context"

	"github.com/jterrazz/studio-station/src/core/call"
	"github.com/jterrazz/studio-station/src/core/machine"
	"github.com/jterrazz/studio-station/src/images"
)

type Deps struct {
	Images Images
	Kind   machine.Kind
}

type Images interface {
	List(ctx context.Context) ([]images.Report, error)
	// value: builds the named images, or every one missing or stale; each
	// step's label goes to step as it starts.
	Build(ctx context.Context, names []string, binary string, step func(label string)) ([]images.Record, error)
	CheckDelete(image, version string) error
	Delete(ctx context.Context, image, version string) ([]images.Record, error)
}

func All(d Deps) []call.Handler {
	return []call.Handler{call.Bind(List, d), call.Bind(Build, d), call.Bind(Delete, d)}
}
