package kit

import (
	"errors"

	"github.com/jterrazz/studio-station/src/domain/machines"

	"github.com/jterrazz/studio-station/src/core/fail"
	"github.com/jterrazz/studio-station/src/domain/setup"
)

func setupFailure(err error) (*fail.Error, bool) {
	var mismatch *setup.NameMismatchError
	var bad *setup.BadNameError
	var step *setup.StepFailedError
	var different *setup.DifferentContentError
	var busy *setup.BusyError
	var exists *setup.DestinationExistsError
	var cross *setup.CrossDeviceError
	switch {
	case errors.As(err, &mismatch):
		return fail.Usage(mismatch.Error(), "run station setup without --name"), true
	case errors.As(err, &bad):
		return fail.Usage(bad.Error(), "use lowercase letters, digits and -, 1 to 40 characters"), true
	case errors.Is(err, setup.ErrNameOnVM):
		return fail.Usage(setup.ErrNameOnVM.Error(), "run station setup without --name"), true
	case errors.As(err, &step):
		hint := "fix it, then run station setup again: it resumes"
		var underAgent *machines.RunningUnderAgentError
		switch {
		case errors.As(err, &underAgent):
			hint = "stop the lasting VMs and delete the temporary ones (station machines), then station setup"
		case errors.As(err, &different):
			hint = "keep one of the two files, then run station setup again"
		case errors.As(err, &busy):
			hint = "stop the lasting VMs and delete the temporary ones (station machines), then station setup"
		case errors.As(err, &exists):
			hint = exists.Path + " already exists: move one of them by hand"
		case errors.As(err, &cross):
			hint = "station never copies: move it by hand, then station setup"
		}
		return fail.Failed(step.Error(), hint, err), true
	}
	return nil, false
}
