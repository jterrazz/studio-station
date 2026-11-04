package words

import "github.com/jterrazz/studio-station/src/core/fail"

const UnknownCommand fail.Code = "usage.unknown-command"

func usage() map[fail.Code]Wording {
	return map[fail.Code]Wording{
		UnknownCommand: func(e *fail.Error) (string, string) {
			return "unknown command \"" + e.Subject + "\"", "station help"
		},
	}
}
