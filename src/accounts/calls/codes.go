package calls

import (
	"errors"

	"github.com/jterrazz/studio-station/src/accounts"
	"github.com/jterrazz/studio-station/src/core/fail"
)

const (
	Unknown      fail.Code = "account.unknown"
	NotShareable fail.Code = "account.not-shareable"
	NotSignedIn  fail.Code = "account.not-signed-in"
	NotShared    fail.Code = "account.not-shared"
	Unreadable   fail.Code = "account.unreadable"
	VMStopped    fail.Code = "account.vm-stopped"
	UnknownVM    fail.Code = "account.unknown-vm"
	// value: refused by the surface, which knows the fleet: a VM of another
	// Mac, and a server.
	OtherMac fail.Code = "account.other-mac"
	ToServer fail.Code = "account.to-server"
)

func coded(err error) error {
	// invariant: the refusals of the Accounts calls travel as their codes,
	// worded by the binary that prints them; no cause carries a secret.
	var unknown *accounts.UnknownError
	var notShareable *accounts.NotShareableError
	var notSignedIn *accounts.NotSignedInError
	var notShared *accounts.NotSharedError
	var unreadable *accounts.UnreadableError
	var stopped *accounts.VMStoppedError
	var unknownVM *accounts.UnknownVMError
	switch {
	case errors.As(err, &unknown):
		return fail.New(fail.NotFound, Unknown, unknown.Account, nil)
	case errors.As(err, &notShareable):
		return fail.New(fail.Refused, NotShareable, notShareable.Account, nil)
	case errors.As(err, &notSignedIn):
		return fail.New(fail.Refused, NotSignedIn, notSignedIn.Account, map[string]string{"mac": notSignedIn.Mac})
	case errors.As(err, &notShared):
		return fail.New(fail.NotFound, NotShared, notShared.Account, map[string]string{"vm": notShared.VM})
	case errors.As(err, &unreadable):
		return &fail.Error{Type: fail.Failed, Code: Unreadable, Subject: unreadable.Account, Detail: map[string]string{"mac": unreadable.Mac}, Cause: unreadable.Cause}
	case errors.As(err, &stopped):
		return fail.New(fail.Refused, VMStopped, stopped.VM, nil)
	case errors.As(err, &unknownVM):
		return fail.New(fail.NotFound, UnknownVM, unknownVM.VM, nil)
	}
	return err
}
