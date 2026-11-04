package words

import (
	"github.com/jterrazz/studio-station/src/core/call"
	"github.com/jterrazz/studio-station/src/core/confirm"
	"github.com/jterrazz/studio-station/src/core/fail"
)

type Wording func(e *fail.Error) (message, hint string)

func Codes() map[fail.Code]Wording {
	// invariant: the one table of every error code's words (rule
	// every-code-worded): a code raised anywhere is worded here once.
	return map[fail.Code]Wording{
		confirm.Needed: func(e *fail.Error) (string, string) {
			return "refusing without the name typed: " + e.Subject, "re-run with --yes"
		},
		confirm.Mismatch: func(e *fail.Error) (string, string) {
			return "\"" + e.Detail["typed"] + "\" is not " + e.Subject, "type the name exactly: " + e.Subject
		},
		confirm.YesNeeded: func(e *fail.Error) (string, string) {
			return "refusing without --yes: " + e.Subject, "re-run with --yes"
		},
		call.NotForThisMachine: func(e *fail.Error) (string, string) {
			if why := e.Detail["why"]; why != "" {
				return why, "run station status for what " + e.Subject + " has"
			}
			return "a " + e.Detail["kind"] + " has no " + e.Detail["section"] + " section", "run station status for what " + e.Subject + " has"
		},
		call.BadInput: func(e *fail.Error) (string, string) {
			return e.Subject + " cannot read its input", "check the request against " + e.Detail["in"]
		},
		call.BadOutput: func(e *fail.Error) (string, string) {
			return e.Subject + " answered what it cannot write", "report it: station --version"
		},
		call.Unknown: func(e *fail.Error) (string, string) {
			return "no call is named " + e.Subject, "run station --help"
		},
		call.Elsewhere: func(e *fail.Error) (string, string) {
			return e.Subject + " is not reached by a call yet", "run it there: station --on " + e.Subject + " ..."
		},
	}
}
