package kit

import (
	"errors"

	"github.com/jterrazz/studio-station/src/core/fail"
	"github.com/jterrazz/studio-station/src/domain/secrets"
)

func secretsFailure(err error) (*fail.Error, bool) {
	var notSession *secrets.NotSessionError
	var notListed *secrets.NotOnLendListError
	var notLent *secrets.NotLentError
	var unknown *secrets.UnknownVMError
	var notVM *secrets.NotAVMError
	var offHost *secrets.OffHostError
	var empty *secrets.EmptyLendListError
	var noStudio *secrets.NoStudioError
	var step *secrets.StepError
	switch {
	case errors.As(err, &notSession):
		return usage(notSession.Error(), "other secrets wait for the secret manager (TODO.md)"), true
	case errors.As(err, &notListed):
		return usage(notListed.Error(), "station secrets lists the lend list; station secrets allow "+notListed.Name+" adds it"), true
	case errors.As(err, &notLent):
		return usage(notLent.Error(), "station secrets lists what was lent to which VM"), true
	case errors.As(err, &unknown):
		return usage(unknown.Error(), "station machines lists the VMs"), true
	case errors.As(err, &notVM):
		return usage(notVM.Error(), "station machines lists the VMs"), true
	case errors.As(err, &offHost):
		return usage(offHost.Error(), "run it on a host: station secrets <verb> --on <host>"), true
	case errors.As(err, &empty):
		return usage(empty.Error(), "station secrets allow session:<id>"), true
	case errors.As(err, &noStudio):
		return failed(noStudio.Error(), "see docs/reference/secrets.md#studio-in-a-vm", err), true
	case errors.As(err, &step):
		return failed(step.Error(), "check the VM: station machines probe "+step.VM, err), true
	}
	return nil, false
}
