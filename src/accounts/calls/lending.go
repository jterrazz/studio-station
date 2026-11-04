package calls

import (
	"context"
	"strings"
	"time"

	"github.com/jterrazz/studio-station/src/accounts"
	"github.com/jterrazz/studio-station/src/core/call"
	"github.com/jterrazz/studio-station/src/core/confirm"
	"github.com/jterrazz/studio-station/src/core/fail"
	"github.com/jterrazz/studio-station/src/core/section"
)

type ListIn struct{}

var List = call.Call[Deps, ListIn, accounts.Lending]{
	Name: "secrets.list", Section: section.Accounts, Where: call.AtTarget, Applies: accounts.Applies,
	Run: func(ctx context.Context, d Deps, _ ListIn, _ call.Progress) (accounts.Lending, error) {
		return d.Secrets.List(ctx)
	},
}

type NamesIn struct{ Names []string }

var Allow = call.Call[Deps, NamesIn, accounts.RecallReport]{
	Name: "secrets.allow", Section: section.Accounts, Where: call.AtTarget, Applies: accounts.Applies,
	Run: func(_ context.Context, d Deps, in NamesIn, _ call.Progress) (accounts.RecallReport, error) {
		return accounts.RecallReport{}, d.Secrets.Allow(in.Names)
	},
}

type DisallowIn struct {
	Names   []string
	Confirm string
}

var Disallow = call.Call[Deps, DisallowIn, accounts.RecallReport]{
	Name: "secrets.disallow", Section: section.Accounts, Where: call.InEngine, Applies: accounts.Applies, Confirm: confirm.TakeBack,
	Errors: []fail.Code{confirm.YesNeeded},
	Run: func(ctx context.Context, d Deps, in DisallowIn, _ call.Progress) (accounts.RecallReport, error) {
		// invariant: a name lent nowhere comes off the list without a
		// question; one lent to a VM asks first, since its copies are recalled.
		if err := d.Secrets.CheckDisallow(in.Names); err != nil {
			return accounts.RecallReport{}, err
		}
		held, err := d.Secrets.Held(in.Names)
		if err != nil {
			return accounts.RecallReport{}, err
		}
		if len(held) > 0 {
			if err := confirm.Require(confirm.For(confirm.TakeBack, d.Kind), strings.Join(in.Names, ", "), in.Confirm); err != nil {
				return accounts.RecallReport{}, err
			}
		}
		return d.Secrets.Disallow(ctx, in.Names)
	},
}

type LendIn struct {
	To    string
	Names []string
	Term  time.Duration
}

var Lend = call.Call[Deps, LendIn, accounts.LendReport]{
	Name: "secrets.lend", Section: section.Accounts, Where: call.InEngine, Applies: accounts.Applies,
	Run: func(ctx context.Context, d Deps, in LendIn, _ call.Progress) (accounts.LendReport, error) {
		return d.Secrets.Lend(ctx, in.To, in.Names, in.Term)
	},
}

type RecallIn struct {
	From    string
	Names   []string
	Confirm string
}

var Recall = call.Call[Deps, RecallIn, accounts.RecallReport]{
	Name: "secrets.recall", Section: section.Accounts, Where: call.InEngine, Applies: accounts.Applies, Confirm: confirm.TakeBack,
	Errors: []fail.Code{confirm.YesNeeded},
	Run: func(ctx context.Context, d Deps, in RecallIn, _ call.Progress) (accounts.RecallReport, error) {
		// invariant: a VM that holds nothing to take back is answered without
		// a question; one that holds a copy asks first, since it is deleted.
		names, err := d.Secrets.CheckRecall(ctx, in.From, in.Names)
		if err != nil {
			return accounts.RecallReport{}, err
		}
		if len(names) > 0 {
			if err := confirm.Require(confirm.For(confirm.TakeBack, d.Kind), strings.Join(names, ", "), in.Confirm); err != nil {
				return accounts.RecallReport{}, err
			}
		}
		return d.Secrets.Recall(ctx, in.From, in.Names)
	},
}
