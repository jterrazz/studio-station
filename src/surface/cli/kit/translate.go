package kit

import (
	"context"
	"errors"
	"strings"

	"github.com/jterrazz/studio-station/src/core/fail"
	"github.com/jterrazz/studio-station/src/surface/cli/words"
)

func Translate(err error, path string) *fail.Error {
	if err == nil {
		return nil
	}
	hint := "run " + strings.TrimSpace("station "+path) + " --help"
	var worded *fail.Error
	if errors.As(err, &worded) {
		if word, ok := words.Codes()[worded.Code]; ok && worded.Message == "" {
			worded.Message, worded.Hint = word(worded)
		}
		var said *fail.Said
		if worded.Message == "" && errors.As(worded.Cause, &said) {
			worded.Message, worded.Hint = said.Message, said.Hint
		}
		if worded.Message != "" && worded.Hint == "" {
			worded.Hint = hint
		}
		return worded
	}
	for _, failure := range []func(error) (*fail.Error, bool){
		toolsFailure,
		configFailure,
		setupFailure,
		machinesFailure,
		imagesFailure,
		secretsFailure,
		tailnetFailure,
		powerFailure,
		engineFailure,
		saidFailure,
		settingsFailure,
	} {
		if f, ok := failure(err); ok {
			return f
		}
	}
	if errors.Is(err, context.Canceled) {
		return fail.Interruption()
	}
	message := err.Error()
	if strings.HasPrefix(message, "unknown command ") {
		first, suggestions, _ := strings.Cut(message, "\n\nDid you mean this?\n")
		if word, ok := strings.CutSuffix(strings.TrimSpace(first), " for \"station\""); ok {
			return Translate(fail.New(fail.Refused, words.UnknownCommand, strings.Trim(strings.TrimPrefix(word, "unknown command "), "\""), nil), path)
		}
		if s := strings.Fields(suggestions); len(s) > 0 {
			hint += didYouMean(s)
		}
		return usage(strings.TrimSpace(first), hint)
	}
	return failed(message, hint, err)
}

func AdminApply(setting, mac string, here bool) string {
	// value: the command that applies an admin setting on that Mac's own
	// terminal, in the words' table.
	return words.AdminApply(setting, mac, here)
}
