package kit

import (
	"errors"
	"github.com/jterrazz/studio-station/src/core/fail"
	"github.com/jterrazz/studio-station/src/domain/tools"
)

func toolsFailure(err error) (*fail.Error, bool) {
	var irreversible *tools.NotReversibleError
	var manual *tools.ManualInstallError
	var noUpgrade *tools.NoUpgradePathError
	var appStore *tools.AppStoreError
	switch {
	case errors.As(err, &irreversible):
		return usage(irreversible.Error(), "station installs it but cannot remove it: remove it by hand"), true
	case errors.As(err, &manual):
		return usage(manual.Error(), manual.Hint), true
	case errors.As(err, &appStore):
		return usage(appStore.Error(), "remove it from /Applications"), true
	case errors.As(err, &noUpgrade):
		return usage(noUpgrade.Error(), "reinstall it by hand: station apps install "+noUpgrade.Tool), true
	}
	return nil, false
}
