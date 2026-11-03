package confirm

import (
	"github.com/jterrazz/studio-station/src/core/fail"
	"github.com/jterrazz/studio-station/src/core/machine"
)

type Action string

const (
	Install     Action = "install"
	Share       Action = "share"
	Start       Action = "start"
	Restart     Action = "restart"
	Edit        Action = "edit"
	Remove      Action = "remove"
	TakeBack    Action = "take-back"
	StopSharing Action = "stop-sharing"
	OffHere     Action = "off-here"
	Overwrite   Action = "overwrite"
	Forget      Action = "forget"
	DeleteImage Action = "delete-image"
	RestartMac  Action = "restart-mac"
	StopEngine  Action = "stop-engine"
	Stop        Action = "stop"
	Delete      Action = "delete"
	Erase       Action = "erase"
)

type Level string

const (
	None     Level = "none"
	YesNo    Level = "yes-no"
	TypeName Level = "type-name"
)

const Yes = "yes"

const (
	Needed    fail.Code = "confirm.needed"
	Mismatch  fail.Code = "confirm.mismatch"
	YesNeeded fail.Code = "confirm.yes-needed"
)

func For(a Action, k machine.Kind) Level {
	// invariant: SPEC §7.5, the graded confirmations of every surface. An
	// action of no row asks yes or no.
	switch a {
	case Install, Share, Start, Restart, Edit:
		return None
	case Delete, Erase:
		return TypeName
	case Stop:
		if k == machine.Server {
			return TypeName
		}
		return YesNo
	default:
		return YesNo
	}
}

func Require(l Level, name, given string) error {
	// invariant: given is what the caller answered: Yes for a yes-or-no
	// question, the typed name for a typed one. Yes never answers a typed one.
	detail := map[string]string{"level": string(l), "name": name, "typed": given}
	switch l {
	case None:
		return nil
	case TypeName:
		if given == name {
			return nil
		}
		if given == "" || given == Yes {
			return fail.New(fail.NeedsConfirmation, Needed, name, detail)
		}
		return fail.New(fail.NeedsConfirmation, Mismatch, name, detail)
	default:
		if given == Yes {
			return nil
		}
		return fail.New(fail.NeedsConfirmation, YesNeeded, name, detail)
	}
}
