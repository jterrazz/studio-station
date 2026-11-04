package calls

import (
	"context"
	"strings"

	"github.com/jterrazz/studio-station/src/core/call"
	"github.com/jterrazz/studio-station/src/core/confirm"
	"github.com/jterrazz/studio-station/src/core/fail"
	"github.com/jterrazz/studio-station/src/core/section"
	"github.com/jterrazz/studio-station/src/disk"
)

type ResetIn struct {
	Names   []string
	Confirm string
}

var Reset = call.Call[Deps, ResetIn, []disk.Outcome]{
	Name: "storage.reset", Section: section.Disk, Where: call.AtTarget, Applies: disk.Applies, Confirm: confirm.Erase,
	Errors: []fail.Code{confirm.Needed, confirm.Mismatch},
	Run: func(ctx context.Context, d Deps, in ResetIn, _ call.Progress) ([]disk.Outcome, error) {
		if _, err := d.Storage.CheckReset(in.Names); err != nil {
			return nil, err
		}
		if err := confirm.Require(confirm.For(confirm.Erase, d.Kind), strings.Join(in.Names, ", "), in.Confirm); err != nil {
			return nil, err
		}
		return d.Storage.Reset(ctx, in.Names)
	},
}
