package kit

import (
	"errors"

	"github.com/jterrazz/studio-station/src/core/fail"
	"github.com/jterrazz/studio-station/src/domain/secrets"
)

func secretsFailure(err error) (*fail.Error, bool) {
	var notVM *secrets.NotAVMError
	var offMac *secrets.OffHostError
	var noStudio *secrets.NoStudioError
	var step *secrets.StepError
	switch {
	case errors.As(err, &notVM):
		return usage(notVM.Error(), "station machines"), true
	case errors.As(err, &offMac):
		return usage(offMac.Error(), "station accounts share <account> --with <vm> --on <mac>"), true
	case errors.As(err, &noStudio):
		return failed(noStudio.Error(), "see docs/reference/secrets.md#studio-in-a-vm", err), true
	case errors.As(err, &step):
		return failed(step.Error(), "check the VM: station machines "+step.VM, err), true
	}
	return nil, false
}
