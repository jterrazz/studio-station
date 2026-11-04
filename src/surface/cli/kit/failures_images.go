package kit

import (
	"errors"
	"strings"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/core/fail"
	"github.com/jterrazz/studio-station/src/domain/images"
)

const linuxBinaryHint = "make build-linux, then station images build linux --binary .artifacts/go/station-linux-arm64"

func imagesFailure(err error) (*fail.Error, bool) {
	var unknown *images.UnknownImageError
	var version *images.UnknownVersionError
	var notBuilt *images.NotBuiltError
	var noBinary *images.NoLinuxBinaryError
	var missing *images.BinaryNotFoundError
	var noEngine *images.NoEngineError
	var notHost *images.NotAHostError
	var build *images.BuildFailedError
	switch {
	case errors.As(err, &unknown):
		return usage(unknown.Error(), "the images are "+imageNames()+": station images build "+catalog.Images[0].Name), true
	case errors.As(err, &version):
		return usage(version.Error(), "station images lists the versions"), true
	case errors.As(err, &notBuilt):
		return usage(notBuilt.Error(), "station images build "+notBuilt.Image), true
	case errors.As(err, &noBinary):
		return usage(noBinary.Error(), linuxBinaryHint), true
	case errors.As(err, &missing):
		return usage(missing.Error(), linuxBinaryHint), true
	case errors.As(err, &noEngine):
		return usage(noEngine.Error(), "station apps install "+noEngine.Hypervisor), true
	case errors.As(err, &notHost):
		return usage(notHost.Error(), "run it on a host"), true
	case errors.As(err, &build):
		return failed(build.Error(), "station images build "+build.Image, err), true
	}
	return nil, false
}

func imageNames() string {
	names := make([]string, 0, len(catalog.Images))
	for _, img := range catalog.Images {
		names = append(names, img.Name)
	}
	return strings.Join(names, ", ")
}
