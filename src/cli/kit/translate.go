package kit

import (
	"context"
	"errors"
	"strings"

	"github.com/jterrazz/studio-station/src/fail"
)

func Translate(err error, path string) *fail.Error {
	if err == nil {
		return nil
	}
	hint := "run " + strings.TrimSpace("station "+path) + " --help"
	var worded *fail.Error
	if errors.As(err, &worded) {
		if worded.Message != "" && worded.Hint == "" {
			worded.Hint = hint
		}
		return worded
	}
	for _, failure := range []func(error) (*fail.Error, bool){
		toolsFailure,
		configFailure,
		setupFailure,
		cleanFailure,
		machinesFailure,
		imagesFailure,
		secretsFailure,
		remoteFailure,
		tailnetFailure,
		shortcutsFailure,
		settingsFailure,
	} {
		if f, ok := failure(err); ok {
			return f
		}
	}
	if errors.Is(err, context.Canceled) {
		return fail.Interrupted()
	}
	message := err.Error()
	if strings.HasPrefix(message, "unknown command ") {
		first, suggestions, _ := strings.Cut(message, "\n\nDid you mean this?\n")
		if s := strings.Fields(suggestions); len(s) > 0 {
			hint += didYouMean(s)
		}
		return fail.Usage(strings.TrimSpace(first), hint)
	}
	return fail.Failed(message, hint, err)
}
