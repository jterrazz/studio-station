package section

import "github.com/jterrazz/studio-station/src/core/machine"

type ID string

const (
	Overview   ID = "overview"
	Activity   ID = "activity"
	Apps       ID = "apps"
	Settings   ID = "settings"
	Disk       ID = "disk"
	Ports      ID = "ports"
	Accounts   ID = "accounts"
	Background ID = "background"
	Awake      ID = "awake"
	SSH        ID = "ssh"
	Images     ID = "images"
)

func Order() []ID {
	// invariant: the one order of the sections, on every surface.
	return []ID{Overview, Activity, Apps, Settings, Disk, Ports, Accounts, Background, Awake, SSH, Images}
}

func For(k machine.Kind) []ID {
	// invariant: the table of SPEC §3.2. Images is an All machines section, and
	// a workspace's Background still needs its job manager (background's rule).
	switch k {
	case machine.Mac:
		return []ID{Overview, Activity, Apps, Settings, Disk, Ports, Accounts, Background, Awake}
	case machine.Workspace:
		return []ID{Overview, Activity, Apps, Settings, Disk, Ports, Accounts, Background}
	case machine.Server:
		return []ID{Overview, Activity, Ports, SSH}
	default:
		return nil
	}
}
