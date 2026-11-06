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
	Lends          []Lend
}

type VMRead struct {
	Name     string    `json:"name"`
	Guest    string    `json:"guest"`
	Accounts []Account `json:"accounts"`
}

func Redact(a Account) Account {
	clean := func(v string) string {
		c, redacted := Clean(v)
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
