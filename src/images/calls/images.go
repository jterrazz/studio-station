package calls

import (
	"context"

	"github.com/jterrazz/studio-station/src/core/call"
	"github.com/jterrazz/studio-station/src/core/confirm"
	"github.com/jterrazz/studio-station/src/core/fail"
	"github.com/jterrazz/studio-station/src/core/machine"
	"github.com/jterrazz/studio-station/src/images"
)

type ListIn struct{}

var List = call.Call[Deps, ListIn, []images.Report]{
	Name: "images.list", Where: call.AtTarget, Applies: machine.Anywhere,
	Run: func(ctx context.Context, d Deps, _ ListIn, _ call.Progress) ([]images.Report, error) {
		return d.Images.List(ctx)
	},
}

type BuildIn struct {
	// value: no name builds every image missing or whose recipe changed.
	Names []string
	// value: the linux station binary the image carries; empty takes the
	// release's.
	Binary string
}

var Build = call.Call[Deps, BuildIn, []images.Record]{
	Name: "images.build", Where: call.InEngine, Applies: machine.Anywhere,
	Run: func(ctx context.Context, d Deps, in BuildIn, p call.Progress) ([]images.Record, error) {
		last := ""
		records, err := d.Images.Build(ctx, in.Names, in.Binary, func(label string) {
			if last != "" {
				p.Done(last, nil)
			}
			last = label
			p.Step(label, "")
		})
		if last != "" {
			p.Done(last, err)
		}
		return records, err
	},
}

type DeleteIn struct {
	Image string
	// value: empty deletes every version of the image.
	Version string
	Confirm string
}

var Delete = call.Call[Deps, DeleteIn, []images.Record]{
	Name: "images.delete", Where: call.InEngine, Applies: machine.Anywhere, Confirm: confirm.DeleteImage,
	Errors: []fail.Code{confirm.YesNeeded},
	Run: func(ctx context.Context, d Deps, in DeleteIn, _ call.Progress) ([]images.Record, error) {
		if err := d.Images.CheckDelete(in.Image, in.Version); err != nil {
			return nil, err
		}
		name := in.Image
		if in.Version != "" {
			name += "@" + in.Version
		}
		if err := confirm.Require(confirm.For(confirm.DeleteImage, d.Kind), name, in.Confirm); err != nil {
			return nil, err
		}
		return d.Images.Delete(ctx, in.Image, in.Version)
	},
}
