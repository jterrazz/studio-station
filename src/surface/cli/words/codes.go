package words

import (
	"cmp"
	"strings"

	appscalls "github.com/jterrazz/studio-station/src/apps/calls"
	"github.com/jterrazz/studio-station/src/core/call"
	"github.com/jterrazz/studio-station/src/core/confirm"
	"github.com/jterrazz/studio-station/src/core/fail"
	diskcalls "github.com/jterrazz/studio-station/src/disk/calls"
	enginecalls "github.com/jterrazz/studio-station/src/engine/calls"
	imagescalls "github.com/jterrazz/studio-station/src/images/calls"
	settingscalls "github.com/jterrazz/studio-station/src/settings/calls"
)

type Wording func(e *fail.Error) (message, hint string)

func Codes() map[fail.Code]Wording {
	// invariant: the one table of every error code's words (rule
	// every-code-worded): a code raised anywhere is worded here once.
	all := map[fail.Code]Wording{
		confirm.Needed: func(e *fail.Error) (string, string) {
			return "refusing without the name typed: " + e.Subject, "in a script: --confirm=" + e.Subject
		},
		confirm.Mismatch: func(e *fail.Error) (string, string) {
			return "you typed \"" + e.Detail["typed"] + "\", not \"" + e.Subject + "\"", "in a script: --confirm=" + e.Subject
		},
		confirm.YesNeeded: func(e *fail.Error) (string, string) {
			return "refusing without --yes: " + e.Subject, "re-run with --yes"
		},
		confirm.Declined: func(e *fail.Error) (string, string) {
			return e.Subject + " kept: you said no", "--yes skips the question"
		},
		call.NotForThisMachine: func(e *fail.Error) (string, string) {
			if why, fix := e.Detail["why"], e.Detail["fix"]; why != "" && fix != "" {
				return why, fix
			}
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
			return cmp.Or(e.Detail["label"], e.Subject) + " does not answer", "station machines"
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
		diskcalls.Unknown: func(e *fail.Error) (string, string) {
			return "no disk row for \"" + e.Subject + "\"", "station disk"
		},
		diskcalls.Elsewhere: func(e *fail.Error) (string, string) {
			return e.Detail["why"], "station disk"
		},
		diskcalls.NothingToFree: func(e *fail.Error) (string, string) {
			return e.Subject + " keeps nothing station can rebuild", "station disk"
		},
		diskcalls.NeverTouched: func(e *fail.Error) (string, string) {
			return e.Detail["path"] + " is never touched: station erases no key, token or setting", "station disk"
		},
		diskcalls.NothingToErase: func(e *fail.Error) (string, string) {
			return e.Subject + " has no data station can erase", cmp.Or(e.Detail["hint"], "station disk")
		},
		diskcalls.AppRunning: func(e *fail.Error) (string, string) {
			process, subject := e.Detail["process"], e.Detail["subject"]
			if e.Detail["verb"] == "move" {
				return process + " is running: " + subject + " cannot move now", "quit " + process + ", then station disk move " + subject
			}
			return process + " is running: its data cannot be erased now", "quit " + process + ", then station disk erase " + subject
		},
		diskcalls.NotMisplaced: func(e *fail.Error) (string, string) {
			return e.Subject + " is not misplaced", "station disk"
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
