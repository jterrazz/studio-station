package calls

import (
	"errors"

	"github.com/jterrazz/studio-station/src/apps"
	"github.com/jterrazz/studio-station/src/core/fail"
)

const (
	Unknown      fail.Code = "apps.unknown"
	Elsewhere    fail.Code = "apps.lives-elsewhere"
	Needs        fail.Code = "apps.needs"
	NotInstalled fail.Code = "apps.not-installed"
)

func coded(err error) error {
	// invariant: the refusals of the Apps calls travel as their codes, worded
	// by the binary that prints them.
	var unknown *apps.UnknownError
	var elsewhere *apps.ElsewhereError
	var needs *apps.NeedsError
	var absent *apps.NotInstalledError
	switch {
	case errors.As(err, &unknown):
		return fail.New(fail.NotFound, Unknown, unknown.Name, nil)
	case errors.As(err, &elsewhere):
		return fail.New(fail.Refused, Elsewhere, elsewhere.App, map[string]string{"lives": apps.LivesOn(elsewhere.Places), "vm": elsewhere.VM})
	case errors.As(err, &needs):
		return &fail.Error{Type: fail.Failed, Code: Needs, Subject: needs.App, Detail: map[string]string{"needs": needs.Needs}, Cause: err}
	case errors.As(err, &absent):
		return fail.New(fail.Refused, NotInstalled, absent.App, nil)
	}
	return err
}
