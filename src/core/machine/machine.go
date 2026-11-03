package machine

type Kind string

const (
	Mac       Kind = "mac"
	Server    Kind = "server"
	Workspace Kind = "workspace"
)

type OS string

const (
	MacOS OS = "macos"
	Linux OS = "linux"
)

type System struct {
	OS      OS
	Version string
	Arch    string
}

type Ref struct {
	Name string
	// invariant: the name of the Mac a VM runs on, and empty for a Mac.
	On string
}

type Capabilities struct {
	// invariant: each capability is reported by the one contract that owns
	// the fact, never derived from a model name.
	Battery bool
	Lid     bool
	Thermal bool
	Jobs    bool
	VMs     bool
	Engine  bool
}

type Hardware struct {
	Model  string
	Memory int64
	CPUs   int
}

type Profile struct {
	Ref      Ref
	Kind     Kind
	System   System
	Hardware Hardware
	Caps     Capabilities
}

type Verdict struct {
	OK  bool
	Why string
}

var Yes = Verdict{OK: true}

func No(why string) Verdict { return Verdict{Why: why} }

type Applies func(Profile) Verdict

func All(rules ...Applies) Applies {
	return func(p Profile) Verdict {
		for _, rule := range rules {
			if v := rule(p); !v.OK {
				return v
			}
		}
		return Yes
	}
}
