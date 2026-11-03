package kit

import (
	"errors"
	"slices"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/core/fail"
	"github.com/jterrazz/studio-station/src/domain/tools"
)

func toolsFailure(err error) (*fail.Error, bool) {
	var unknown *tools.UnknownToolError
	var place *tools.WrongPlaceError
	var missing *tools.MissingDependencyError
	var irreversible *tools.NotReversibleError
	var manual *tools.ManualInstallError
	var noUpgrade *tools.NoUpgradePathError
	var appStore *tools.AppStoreError
	switch {
	case errors.As(err, &unknown):
		return fail.Usage(unknown.Error(), "run station tools for the catalogue"), true
	case errors.As(err, &place):
		return fail.Usage(place.Error(), placeHint(place)), true
	case errors.As(err, &missing):
		return fail.Failure(missing.Error(), "station tools install "+missing.Requires+" "+missing.Tool, err), true
	case errors.As(err, &irreversible):
		return fail.Usage(irreversible.Error(), "station installs it but cannot remove it: remove it by hand"), true
	case errors.As(err, &manual):
		return fail.Usage(manual.Error(), manual.Hint), true
	case errors.As(err, &appStore):
		return fail.Usage(appStore.Error(), "remove it from /Applications"), true
	case errors.As(err, &noUpgrade):
		return fail.Usage(noUpgrade.Error(), "reinstall it by hand: station tools install "+noUpgrade.Tool), true
	}
	return nil, false
}

func placeHint(e *tools.WrongPlaceError) string {
	switch {
	case e.VM != "":
		return "station tools install " + e.Tool + " --on " + e.VM
	case slices.Contains(e.Places, catalog.Linux):
		return "station machines create"
	case slices.Contains(e.Places, catalog.MacOS):
		return `the macOS VM does not exist yet (TODO.md "The macOS VM")`
	}
	return "run it on a host: station tools install " + e.Tool + " --on <host>"
}
