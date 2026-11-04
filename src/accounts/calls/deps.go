package calls

import (
	"context"
	"time"

	"github.com/jterrazz/studio-station/src/accounts"
	"github.com/jterrazz/studio-station/src/core/call"
	"github.com/jterrazz/studio-station/src/core/machine"
)

type Deps struct {
	Accounts Accounts
	Secrets  Secrets
	Kind     machine.Kind
}

type Accounts interface {
	Read(ctx context.Context) <-chan accounts.Report
}

type Secrets interface {
	List(ctx context.Context) (accounts.Lending, error)
	Allow(names []string) error
	CheckDisallow(names []string) error
	// value: the VMs each name is lent to now.
	Held(names []string) (map[string][]string, error)
	Disallow(ctx context.Context, names []string) (accounts.RecallReport, error)
	Lend(ctx context.Context, to string, names []string, term time.Duration) (accounts.LendReport, error)
	// value: the names a recall from that VM would take back, every name
	// lent to it when none is named.
	CheckRecall(ctx context.Context, from string, names []string) ([]string, error)
	Recall(ctx context.Context, from string, names []string) (accounts.RecallReport, error)
}

func All(d Deps) []call.Handler {
	return []call.Handler{
		call.BindWatch(Read, d), call.Bind(List, d), call.Bind(Allow, d), call.Bind(Disallow, d),
		call.Bind(Lend, d), call.Bind(Recall, d),
	}
}
