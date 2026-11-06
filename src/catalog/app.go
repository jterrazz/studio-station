package catalog

type MachineKind string

const (
	Host  MachineKind = "host"
	Linux MachineKind = "linux"
	MacOS MachineKind = "macos"
)

func (k MachineKind) Article() string {
	if k == Host {
		return "a host"
	}
	return "a " + string(k) + " VM"
}

type LivesOn struct {
	Kinds   []MachineKind
	Install Install
}

type Probe struct {
	Args    []string
	Pattern string
	Path    string
}

type Liveness struct {
	Process string
	Command []string
}

type Tool struct {
	Name        string
	Description string
	Category    Category
	Replaces    string
	Supersedes  string
	AppStore    bool
	Installs    []LivesOn
	Provides    []string
	Bundle      string
	Version     Probe
	Requires    []string
	Then        []string
	Live        Liveness
	Storage     Disk
	System      bool
}

// invariant: which machine kinds accept a method is the `kinds` column of the
// method table (src/domain/tools/methods.go reads it from here), so a placement and
// the dispatch that runs it can never disagree.

var MethodKinds = map[Method][]MachineKind{
	MethodBrew:   {Host, MacOS},
	MethodCask:   {Host, MacOS},
	MethodMas:    {Host, MacOS},
	MethodApp:    {Host, MacOS},
	MethodNpm:    {Host, MacOS},
	MethodUv:     {Host, MacOS},
	MethodBun:    {Host, Linux, MacOS},
	MethodMise:   {Host, Linux, MacOS},
	MethodScript: {Host, Linux, MacOS},
	MethodShell:  {Host, Linux, MacOS},
	MethodSkill:  {Host, Linux, MacOS},
}

func Everywhere(mac, linux Install) []LivesOn {
	return []LivesOn{{Kinds: []MachineKind{Host, MacOS}, Install: mac}, {Kinds: []MachineKind{Linux}, Install: linux}}
}

func Mac(i Install) []LivesOn { return []LivesOn{{Kinds: []MachineKind{Host, MacOS}, Install: i}} }

func HostOnly(i Install) []LivesOn { return []LivesOn{{Kinds: []MachineKind{Host}, Install: i}} }

func LinuxOnly(i Install) []LivesOn { return []LivesOn{{Kinds: []MachineKind{Linux}, Install: i}} }

func VMs(mac, linux Install) []LivesOn {
	return []LivesOn{{Kinds: []MachineKind{MacOS}, Install: mac}, {Kinds: []MachineKind{Linux}, Install: linux}}
}

func (t Tool) LivesOn() []MachineKind {
	var kinds []MachineKind
	for _, k := range []MachineKind{Host, Linux, MacOS} {
		if _, ok := t.InstallOn(k); ok {
			kinds = append(kinds, k)
		}
	}
	return kinds
}

const AppStoreReceipt = "Contents/_MASReceipt"

func (t Tool) Superseded() (Install, bool) {
	// invariant: what a tool supersedes is a formula, named by Supersedes.
	if t.Supersedes == "" {
		return nil, false
	}
	return Brew(t.Supersedes), true
}

func (t Tool) InstallOn(k MachineKind) (Install, bool) {
	for _, p := range t.Installs {
		for _, kind := range p.Kinds {
			if kind == k {
				return p.Install, true
			}
		}
	}
	return nil, false
}

func Anywhere(i Install) []LivesOn {
	return []LivesOn{{Kinds: []MachineKind{Host, Linux, MacOS}, Install: i}}
}
