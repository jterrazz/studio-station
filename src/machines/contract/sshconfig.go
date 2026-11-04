package contract

import "context"

type HostBlock struct {
	Name, User string
	// value: the private key file; empty is the owner's default key.
	Key string
	// value: the Mac a VM runs on, empty for a Mac; Here when that Mac is this one.
	Behind string
	Here   bool
}

type SSHConfigError struct {
	Path  string
	Cause error
}

func (e *SSHConfigError) Error() string { return e.Cause.Error() }

func (e *SSHConfigError) Unwrap() error { return e.Cause }

type SSHConfig interface {
	// invariant: a Host the owner wrote outside station's blocks is never
	// shadowed: its block is refused with an SSHConfigError, the others are
	// still written.
	Put(ctx context.Context, blocks []HostBlock) error
	// value: the blocks are every one station manages; any other block goes.
	Replace(ctx context.Context, blocks []HostBlock) error
	Remove(ctx context.Context, names ...string) error
}
