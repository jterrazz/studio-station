package kit

import (
	"testing"

	"github.com/jterrazz/studio-station/src/core/fail"
)

func usage(message, hint string) *fail.Error {
	guard(hint)
	return &fail.Error{Message: message, Hint: hint, Exit: fail.ExitUsage}
}

func failed(message, hint string, cause error) *fail.Error {
	guard(hint)
	return &fail.Error{Message: message, Hint: hint, Exit: fail.ExitFailure, Cause: cause}
}

func guard(hint string) {
	// invariant: every worded error carries its one repair; under test an empty
	// hint is a bug to fail loudly, at run time Translate fills it in.
	if hint == "" && testing.Testing() {
		panic("kit: an error needs a non-empty hint")
	}
}
