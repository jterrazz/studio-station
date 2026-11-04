package contract

import (
	"context"
	"time"

	"github.com/jterrazz/studio-station/src/activity"
)

type Meter interface {
	// invariant: the busiest processes by CPU and by memory, at most
	// activity.Top each, busiest first; a list no tool answered is empty.
	Sample(ctx context.Context) (activity.Sample, error)
	// invariant: ok is false when the machine did not say when it booted.
	Uptime(ctx context.Context) (up time.Duration, ok bool)
	// value: the battery's level and state ("92% charging"); "" where there is none.
	Battery(ctx context.Context) string
	// value: the memory installed, in bytes; 0 when unread.
	Memory(ctx context.Context) int64
}
