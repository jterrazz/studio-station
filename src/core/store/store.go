package store

import (
	"context"
	"errors"
	"strconv"
)

var ErrNotFound = errors.New("no such record")

type UnreadableError struct {
	Path  string
	Cause error
}

func (e *UnreadableError) Error() string { return "cannot read " + e.Path + ": " + e.Cause.Error() }
func (e *UnreadableError) Unwrap() error { return e.Cause }

type UnwritableError struct {
	Path  string
	Cause error
}

func (e *UnwritableError) Error() string { return "cannot write " + e.Path + ": " + e.Cause.Error() }
func (e *UnwritableError) Unwrap() error { return e.Cause }

type NewerError struct {
	Version, Known int
}

func (e *NewerError) Error() string {
	return "written at version " + strconv.Itoa(e.Version) + " by a newer station, this one knows " + strconv.Itoa(e.Known)
}

type Migrator interface {
	// invariant: brings every record of the store to its current version and
	// leaves a record already there byte for byte; run by setup's layout step
	// before any call reads the store.
	Migrate(ctx context.Context) error
}

func Current(version, known int) (int, error) {
	// invariant: a record with no version field is at version 1, the shape
	// every store had before it carried one, so a store at version 1 writes none.
	if version == 0 {
		version = 1
	}
	if version > known {
		return version, &NewerError{Version: version, Known: known}
	}
	return version, nil
}
