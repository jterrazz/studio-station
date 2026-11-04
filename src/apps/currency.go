package apps

type Currency struct {
	Verdict Verdict
	// value: the version a newer release would bring, when the method says it.
	Newest string
}

type Verdict string

const (
	Current Verdict = "current"
	Behind  Verdict = "behind"
	Unknown Verdict = "unknown"
)
