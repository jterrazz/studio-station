package words

import (
	"strings"

	"github.com/jterrazz/studio-station/src/core/call"
	"github.com/jterrazz/studio-station/src/core/confirm"
	"github.com/jterrazz/studio-station/src/core/fail"
	enginecalls "github.com/jterrazz/studio-station/src/engine/calls"
	settingscalls "github.com/jterrazz/studio-station/src/settings/calls"
)

type Wording func(e *fail.Error) (message, hint string)

func Codes() map[fail.Code]Wording {
	// invariant: the one table of every error code's words (rule
	// every-code-worded): a code raised anywhere is worded here once.
	all := map[fail.Code]Wording{
		confirm.Needed: func(e *fail.Error) (string, string) {
			return "refusing without the name typed: " + e.Subject, "re-run with --yes"
		},
		confirm.Mismatch: func(e *fail.Error) (string, string) {
			return "\"" + e.Detail["typed"] + "\" is not " + e.Subject, "type the name exactly: " + e.Subject
		},
		confirm.YesNeeded: func(e *fail.Error) (string, string) {
			return "refusing without --yes: " + e.Subject, "re-run with --yes"
		},
		confirm.Declined: func(e *fail.Error) (string, string) {
			return e.Subject + " kept: you said no", "--yes skips the question"
		},
		call.NotForThisMachine: func(e *fail.Error) (string, string) {
			if why := e.Detail["why"]; why != "" {
				return why, "run station overview for what " + e.Subject + " has"
			}
			return "a " + e.Detail["kind"] + " has no " + e.Detail["section"] + " section", "run station overview for what " + e.Subject + " has"
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
		call.BadFrame: func(e *fail.Error) (string, string) {
			return "the request is not a " + e.Subject + " frame", "send one JSON object per line with v 2 and a call or a watch"
		},
		call.Unreachable: func(e *fail.Error) (string, string) {
			return "cannot reach \"" + e.Subject + "\"", "station machines probe " + e.Subject
		},
		enginecalls.StartFailed: func(e *fail.Error) (string, string) {
			return "the engine did not start: " + e.Detail["last"], "station engine"
		},
		settingscalls.Unknown: func(e *fail.Error) (string, string) {
			return "no setting named \"" + e.Subject + "\"", "station settings"
		},
		settingscalls.NotForThisMachine: func(e *fail.Error) (string, string) {
			if e.Detail["why"] == "Mac only" {
				return e.Subject + " is for Macs only", "station settings apply " + e.Subject + " --on this-mac"
			}
			return e.Subject + " is for " + strings.TrimSuffix(e.Detail["why"], " only") + " only", "station settings"
		},
		settingscalls.NeedsTerminal: func(e *fail.Error) (string, string) {
			return e.Subject + " asks a value: run it on a terminal", "station settings apply " + e.Subject
		},
		call.FarTooOld: func(e *fail.Error) (string, string) {
			return e.Subject + " runs station " + e.Detail["version"] + ", which has no " + e.Detail["call"], "update station on " + e.Subject + " (scripts/install.sh there)"
		},
	}
	for code, word := range usage() {
		all[code] = word
	}
	return all
}
