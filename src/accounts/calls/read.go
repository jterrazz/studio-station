package calls

import (
	"context"

	"github.com/jterrazz/studio-station/src/accounts"
	"github.com/jterrazz/studio-station/src/core/call"
	"github.com/jterrazz/studio-station/src/core/section"
)

type ReadIn struct{}

var Read = call.Watch[Deps, ReadIn, accounts.Report]{
	Name: "accounts.read", Section: section.Accounts, Where: call.AtTarget, Applies: accounts.Applies,
	Run: func(ctx context.Context, d Deps, _ ReadIn, _ call.Progress) (<-chan call.Update[accounts.Report], error) {
		return call.Updates(ctx, d.Accounts.Read(ctx)), nil
	},
}
