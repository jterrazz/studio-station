package images

import (
	"strings"

	"github.com/jterrazz/studio-station/src/catalog"
)

type UnknownImageError struct{ Image string }

func (e *UnknownImageError) Error() string { return "unknown image \"" + e.Image + "\"" }

type UnknownVersionError struct{ Image, Version string }

func (e *UnknownVersionError) Error() string {
	return "image " + e.Image + " has no version " + e.Version
}

type NotBuiltError struct{ Image string }

func (e *NotBuiltError) Error() string { return "image " + e.Image + " is not built here" }

type NoLinuxBinaryError struct{}

func (e *NoLinuxBinaryError) Error() string {
	return "this station is a dev build: there is no release binary to put in the image"
}

type BinaryNotFoundError struct{ Path string }

func (e *BinaryNotFoundError) Error() string { return e.Path + ": no such file" }

type NoEngineError struct {
	Hypervisor string
	Cause      error
}

func (e *NoEngineError) Error() string { return e.Hypervisor + " is not installed or does not answer" }

func (e *NoEngineError) Unwrap() error { return e.Cause }

type NotAHostError struct{ Here catalog.MachineKind }

func (e *NotAHostError) Error() string {
	return "images live on a host; here is " + e.Here.Article()
}

type BuildFailedError struct {
	Image, Step string
	Cause       error
}

func (e *BuildFailedError) Error() string {
	message := "image " + e.Image + ": " + e.Step + " failed"
	if e.Cause != nil {
		if detail := strings.TrimSpace(e.Cause.Error()); detail != "" {
			message += ": " + detail
		}
	}
	return message
}

func (e *BuildFailedError) Unwrap() error { return e.Cause }
