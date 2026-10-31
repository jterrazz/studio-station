package catalog

type State string

const (
	StateOK       State = "ok"
	StateDegraded State = "degraded"
	StateAbsent   State = "absent"
	StateBroken   State = "broken"
	StateUnknown  State = "unknown"
)
