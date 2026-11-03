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
		return fail.Usage(notSession.Error(), "other secrets wait for the secret manager (TODO.md)"), true
	case errors.As(err, &notListed):
		return fail.Usage(notListed.Error(), "station secrets lists the lend list; station secrets allow "+notListed.Name+" adds it"), true
	case errors.As(err, &notLent):
		return fail.Usage(notLent.Error(), "station secrets lists what was lent to which VM"), true
	case errors.As(err, &unknown):
		return fail.Usage(unknown.Error(), "station machines lists the VMs"), true
	case errors.As(err, &notVM):
		return fail.Usage(notVM.Error(), "station machines lists the VMs"), true
	case errors.As(err, &offHost):
		return fail.Usage(offHost.Error(), "run it on a host: station secrets <verb> --on <host>"), true
	case errors.As(err, &empty):
		return fail.Usage(empty.Error(), "station secrets allow session:<id>"), true
	case errors.As(err, &noStudio):
		return fail.Failure(noStudio.Error(), "see docs/reference/secrets.md#studio-in-a-vm", err), true
	case errors.As(err, &step):
		return fail.Failure(step.Error(), "check the VM: station machines probe "+step.VM, err), true
	}
	return nil, false
}
