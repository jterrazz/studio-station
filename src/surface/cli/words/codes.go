package words

import (
	appscalls "github.com/jterrazz/studio-station/src/apps/calls"
	"github.com/jterrazz/studio-station/src/core/call"
	"github.com/jterrazz/studio-station/src/core/confirm"
	"github.com/jterrazz/studio-station/src/core/fail"
	enginecalls "github.com/jterrazz/studio-station/src/engine/calls"
	imagescalls "github.com/jterrazz/studio-station/src/images/calls"
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
			return e.Subject + " is a " + e.Detail["kind"] + ": station does not manage its " + e.Detail["section"], "run station overview for what " + e.Subject + " has"
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
		imagescalls.Unknown: func(e *fail.Error) (string, string) {
			return "no image \"" + e.Subject + "\"", "station images"
		},
		appscalls.Unknown: func(e *fail.Error) (string, string) {
			return "no app named \"" + e.Subject + "\"", "station apps for every app here"
		},
		appscalls.Elsewhere: func(e *fail.Error) (string, string) {
			hint := "station apps install " + e.Subject + " --on <mac>"
			switch {
			case e.Detail["vm"] != "":
				hint = "station apps install " + e.Subject + " --on " + e.Detail["vm"]
			case e.Detail["lives"] != "Macs only":
				hint = "station machines create"
			}
			return e.Subject + " lives on " + e.Detail["lives"], hint
		},
		appscalls.Needs: func(e *fail.Error) (string, string) {
			return e.Subject + " needs " + e.Detail["needs"] + ", which is not installed", "station apps install " + e.Detail["needs"]
		},
		appscalls.NotInstalled: func(e *fail.Error) (string, string) {
			return e.Subject + " is not installed", "station apps install " + e.Subject
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
