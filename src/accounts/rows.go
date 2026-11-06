package accounts

const (
	OriginOwn     = "own"
	OriginLent    = "lent"
	OriginBoth    = "own+lent"
	OriginUnknown = "?"

	DaemonRunning  = "running"
	DaemonStopped  = "stopped"
	DaemonAbsent   = "not installed"
	GuestConnected = "connected"
	GuestAbsent    = "absent"
)

type VM struct {
	// value: a VM of this host as its record and the daemon's list say.
	Name, Lifetime string
	Running        bool
	Lends          []Loan
}

type VMRead struct {
	Name     string    `json:"name"`
	Guest    string    `json:"guest"`
	Accounts []Account `json:"accounts"`
}

func RedactAccount(a Account) Account {
	clean := func(v string) string {
		c, redacted := Redact(v)
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
	if a.Shared != nil {
		l := *a.Shared
		l.Name = clean(l.Name)
		a.Shared = &l
	}
	return a
}
