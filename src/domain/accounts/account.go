package accounts

import (
	"github.com/jterrazz/studio-station/src/accounts"
	"github.com/jterrazz/studio-station/src/catalog"
)

const (
	ReasonUnset       = accounts.ReasonUnset
	ReasonMissing     = accounts.ReasonMissing
	ReasonRefused     = accounts.ReasonRefused
	ReasonLate        = accounts.ReasonLate
	ReasonFailing     = accounts.ReasonFailing
	ReasonExpiring    = accounts.ReasonExpiring
	ReasonUnreachable = accounts.ReasonUnreachable

	OriginOwn     = "own"
	OriginLent    = "lent"
	OriginBoth    = "own+lent"
	OriginUnknown = "?"

	Unknown = accounts.Unknown
)

type (
	Account = accounts.Account
	Lend    = accounts.Lend
)

// invariant: a provider's own read is the owner's own right once it is
// signed in or failing; absent or late, it is no one's yet.
func own(id accounts.Identity) Account {
	a := Account{Provider: id.Provider, State: id.State, Reason: id.Reason, Identity: id.Who, Scopes: id.Scopes, Source: id.Source, Detail: id.Detail, Notes: id.Notes}
	if id.State == catalog.StateOK || id.State == catalog.StateBroken {
		a.Origin = OriginOwn
	}
	return a
}

func Redact(a Account) Account {
	clean := func(v string) string {
		c, redacted := accounts.Clean(v)
		a.Redacted = a.Redacted || redacted
		return c
	}
	cleanAll := func(vs []string) []string {
		out := make([]string, 0, len(vs))
		for _, v := range vs {
			out = append(out, clean(v))
		}
		return out
	}
	a.Identity, a.Scopes = cleanAll(a.Identity), cleanAll(a.Scopes)
	a.Source, a.Detail, a.Notes = clean(a.Source), clean(a.Detail), cleanAll(a.Notes)
	if a.Lent != nil {
		l := *a.Lent
		l.Name = clean(l.Name)
		a.Lent = &l
	}
	return a
}

func ok(provider string) Account { return own(accounts.SignedIn(provider)) }

func absent(provider, reason, detail string) Account {
	return own(accounts.Absent(provider, reason, detail))
}
