package system

import (
	"context"
	"time"
)

type VMState string

const (
	VMRunning   VMState = "running"
	VMStopped   VMState = "stopped"
	VMSuspended VMState = "suspended"
	VMMissing   VMState = "missing"
)

type VMInfo struct {
	Name   string
	State  VMState
	DiskGB *int
	SizeGB int
}

type Resources struct {
	CPUs        int
	MemoryBytes int64
	DiskBytes   int64
}

type Engine interface {
	Version(ctx context.Context) (string, error)
	List(ctx context.Context) ([]VMInfo, error)
	Pull(ctx context.Context, ref string) error
	Clone(ctx context.Context, from, to string) error
	Rename(ctx context.Context, from, to string) error
	Resize(ctx context.Context, name string, r Resources) error
	RunArgv(name, console string) []string
	Stop(ctx context.Context, name string, wait time.Duration) error
	Delete(ctx context.Context, name string) error
	IP(ctx context.Context, name string, wait time.Duration) (string, error)
	PruneCaches(ctx context.Context) error
}
