package tailscale

import (
	"context"
	"errors"

	"github.com/jterrazz/studio-station/src/accounts"
	"github.com/jterrazz/studio-station/src/accounts/contract"
	"github.com/jterrazz/studio-station/src/catalog"
	portscontract "github.com/jterrazz/studio-station/src/ports/contract"
)

const running = "Running"

// value: the identities of every provider, the tailnet's account read from
// the node itself: who is logged in, on which tailnet, with which client.
type Identities struct {
	tailnet portscontract.Tailnet
	others  contract.Identities
}

var _ contract.Identities = (*Identities)(nil)

func New(tailnet portscontract.Tailnet, others contract.Identities) *Identities {
	return &Identities{tailnet: tailnet, others: others}
}

func (i *Identities) Offers(p catalog.Provider) bool { return i.others.Offers(p) }

func (i *Identities) Read(ctx context.Context, p catalog.Provider) (accounts.Identity, error) {
	if p.ID != catalog.ProviderTailscale {
		return i.others.Read(ctx, p)
	}
	self, err := i.tailnet.Self(ctx)
	var absent *portscontract.Absent
	switch {
	case errors.As(err, &absent):
		return accounts.Absent(p.ID, accounts.ReasonMissing, "not installed"), nil
	case err != nil:
		return accounts.Broken(p.ID, accounts.ReasonFailing, "tailscale status did not answer"), nil
	case self.Sharing.Backend != running:
		return accounts.Absent(p.ID, accounts.ReasonUnset, "not logged in"), nil
	}
	id := accounts.SignedIn(p.ID)
	id.Who, id.Detail = []string{self.Login}, self.Client.Variant
	if self.Login == "" {
		id.Who = []string{accounts.Unknown}
	}
	if self.Network != "" {
		id.Scopes = []string{self.Network}
	}
	return id, nil
}
