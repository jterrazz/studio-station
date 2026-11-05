package world

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/jterrazz/studio-station/src/accounts"
	"github.com/jterrazz/studio-station/src/activity"
	"github.com/jterrazz/studio-station/src/awake"
	"github.com/jterrazz/studio-station/src/background"
	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/core/machine"
	"github.com/jterrazz/studio-station/src/engine"
	"github.com/jterrazz/studio-station/src/images"
	"github.com/jterrazz/studio-station/src/machines"
	"github.com/jterrazz/studio-station/src/ports"
	"github.com/jterrazz/studio-station/src/settings"
)

type World struct {
	// invariant: every fake over the world holds this lock while it reads
	// or writes it, as the engine's goroutines and a command share it.
	mu sync.Mutex

	Now time.Time `json:"now"`
	// value: the Mac the binary runs on, and its owner's login.
	Here string          `json:"here"`
	User string          `json:"user,omitempty"`
	Macs map[string]*Mac `json:"macs"`
	// value: what the engine told the owner, in the order told.
	Told []engine.Notification `json:"told,omitempty"`
	// value: each image's latest version, as its registry answers it.
	Registry map[string]string `json:"registry,omitempty"`
	// value: the failures injected into the fakes, in the order scripted.
	Script []Fault `json:"script,omitempty"`
	// value: every command station asked a Mac of the world to run as root,
	// each refused (SC-03: station never runs sudo).
	AsRoot []string `json:"as_root,omitempty"`
}

type Fault struct {
	// value: the method that fails, "<Contract>.<Method>": "Installer.Install".
	Call string `json:"call"`
	// value: how many more calls fail; zero fails every call.
	Times int `json:"times,omitempty"`
	// value: the next call does its first step, then waits until it is
	// interrupted, as a long install caught by ctrl-c.
	Hold bool `json:"hold,omitempty"`
}

type Mac struct {
	// value: the engine of this Mac; nil when none was ever set up.
	Engine *Engine `json:"engine,omitempty"`
	// value: FileVault waits for its password at pre-boot: nothing of the
	// Mac runs, its engine neither.
	Locked bool `json:"locked,omitempty"`
	// value: out of the tailnet's reach.
	Offline bool `json:"offline,omitempty"`
	// value: a Mac that restarts does not answer until then.
	Back time.Time `json:"back,omitzero"`
	// value: the hypervisor this Mac has: empty, the one station is built
	// with; none, no hypervisor installed; another name, a second fake that
	// stands for another adapter (vz).
	Hypervisor string `json:"hypervisor,omitempty"`
	// value: the station version this Mac runs; empty, the spec binary's.
	Station string `json:"station,omitempty"`
	// value: what station keeps in this Mac's preferences, by group.
	Preferences map[string]json.RawMessage `json:"preferences,omitempty"`
	// value: station's own files as text, by name, while no store has read
	// them: a file edited by hand, a file station v2 left.
	Files map[string]string `json:"files,omitempty"`
	// value: the images the hypervisor holds, by reference.
	Images []string `json:"images,omitempty"`
	// value: a VM freshly cloned from a fetched image, before anything ran
	// in it: its size and the guest it answers with; nil clones a disk whose
	// guest is station's image's.
	Fresh *VM            `json:"fresh,omitempty"`
	VMs   map[string]*VM `json:"vms,omitempty"`

	Jobs map[string]*Job `json:"jobs,omitempty"`
	// value: the job manager's overrides by label: enabled or not.
	Overrides map[string]bool `json:"overrides,omitempty"`
	// value: what station keeps in this Mac's vault, by name.
	Vault map[string][]byte `json:"vault,omitempty"`
	// value: the secrets another tool keeps on this Mac, by name.
	Secrets map[string]string `json:"secrets,omitempty"`
	// value: who each provider says is signed in, by provider; a provider
	// absent here has no row on this Mac.
	Identities map[string]accounts.Identity `json:"identities,omitempty"`
	// value: the facts this Mac answers, by fact; a fact absent here is one
	// it cannot look at.
	Facts map[catalog.Fact]settings.Fact `json:"facts,omitempty"`
	// value: what each setting a procedure writes holds now, by setting:
	// "station's" as station applied it, or the owner's own text.
	Settings map[string]string `json:"settings,omitempty"`
	// value: what Self reads; nil reads a Mac on macOS.
	Self *Self `json:"self,omitempty"`
	// value: what the power hardware reads now, its capabilities with it (a
	// battery, a lid); nil on a machine with no power management.
	Power *Power `json:"power,omitempty"`
	// value: what listens on this Mac, one row per listening address.
	Listening []ports.Listener `json:"listening,omitempty"`
	// value: this Mac's node on the tailnet; nil when it is signed out.
	Tailnet *Node `json:"tailnet,omitempty"`
	// value: each installer's state, by method; a method absent here has its
	// manager installed and holds nothing.
	Apps map[catalog.Method]*Manager `json:"apps,omitempty"`
	// value: the counters that move, read by the meter.
	Meter *Meter `json:"meter,omitempty"`
	// value: the volume and the files on it, by absolute path.
	Disk *Disk `json:"disk,omitempty"`
	// value: the ssh client config's managed Host blocks, by name, and the
	// names defined outside them.
	SSH *SSH `json:"ssh,omitempty"`
	// value: station's own records kept beside the preferences, by store.
	Stores *Stores `json:"stores,omitempty"`
}

type Self struct {
	Kind     machine.Kind     `json:"kind,omitempty"`
	System   machine.System   `json:"system"`
	Hardware machine.Hardware `json:"hardware"`
}

type Power struct {
	awake.Reading
	// value: the awake holds taken now, by name, and whether the lid is held.
	Holds     []string `json:"holds,omitempty"`
	Clamshell bool     `json:"clamshell,omitempty"`
	// value: how many times the display was put to sleep.
	DisplaySleeps int `json:"display_sleeps,omitempty"`
}

type Node struct {
	Self   ports.Self    `json:"self"`
	Shares []ports.Share `json:"shares,omitempty"`
	// value: the host's certificate is asked for and not issued yet.
	CertificatePending bool `json:"certificate_pending,omitempty"`
}

type Manager struct {
	// value: the manager itself is not installed.
	Absent    bool              `json:"absent,omitempty"`
	Installed map[string]string `json:"installed,omitempty"`
	// value: the keys a newer version waits for, with that version.
	Behind map[string]string `json:"behind,omitempty"`
	// value: the version an install of a key brings.
	Offers map[string]string `json:"offers,omitempty"`
}

type Meter struct {
	Sample activity.Sample  `json:"sample"`
	Top    activity.Busiest `json:"top"`
	// value: the processes that run, by pid.
	Processes map[int]activity.Usage `json:"processes,omitempty"`
}

type Disk struct {
	Total int64            `json:"total,omitempty"`
	Free  int64            `json:"free,omitempty"`
	Files map[string]int64 `json:"files,omitempty"`
	// value: the processes that run, by name.
	Running []string `json:"running,omitempty"`
	// value: how late each size answers.
	Late time.Duration `json:"late,omitempty"`
}

type SSH struct {
	Blocks  map[string]HostBlock `json:"blocks,omitempty"`
	Foreign []string             `json:"foreign,omitempty"`
}

type HostBlock struct {
	User   string `json:"user,omitempty"`
	Key    string `json:"key,omitempty"`
	Behind string `json:"behind,omitempty"`
	Here   bool   `json:"here,omitempty"`
}

type Stores struct {
	Images       []images.Record             `json:"images,omitempty"`
	Shares       map[string][]accounts.Share `json:"shares,omitempty"`
	Reservations []ports.Reservation         `json:"reservations,omitempty"`
	Declarations []ports.Declaration         `json:"declarations,omitempty"`
	Applied      map[string]settings.Applied `json:"applied,omitempty"`
}

func (m *Mac) Store() *Stores {
	// invariant: the stores of a Mac, made empty the first time they are asked.
	if m.Stores == nil {
		m.Stores = &Stores{}
	}
	return m.Stores
}

type Engine struct {
	// value: the version the engine runs; empty is the binary's own.
	Version string `json:"version,omitempty"`
	// value: when the engine that runs now started.
	StartedAt time.Time `json:"started_at,omitzero"`
	// value: set up, and stopped by the owner: when, and by what.
	Stopped   bool      `json:"stopped,omitempty"`
	StoppedAt time.Time `json:"stopped_at,omitzero"`
	StoppedBy string    `json:"stopped_by,omitempty"`
	// value: what the engine knows of its VMs between two commands.
	Memory engine.Memory `json:"memory,omitempty"`
	// value: what the engine holds of Awake between two commands.
	Awake *engine.AwakeKept `json:"awake,omitempty"`
	// value: what the exposure job knows between two commands.
	Exposure *engine.ExposureKept `json:"exposure,omitempty"`
}

func (m *Mac) EngineRuns() bool {
	// value: whether this Mac's engine answers now.
	return m.Engine != nil && !m.Engine.Stopped && !m.Locked
}

type VM struct {
	// value: what station keeps of the VM; nil for a disk it has no record of.
	Kept *Kept `json:"kept,omitempty"`
	// value: the hypervisor holds its disk.
	Disk bool          `json:"disk,omitempty"`
	Size machines.Size `json:"size"`
	// value: the pid of its run; zero when it does not run.
	PID int `json:"pid,omitempty"`
	// value: how its last run ended, kept until a run is read.
	Ended *machines.RunEnd `json:"ended,omitempty"`
	// value: when its last run began and when its last run ended, on the
	// world's clock: what a wait between the two took in the world.
	UpAt   time.Time `json:"up_at,omitzero"`
	DownAt time.Time `json:"down_at,omitzero"`
	// value: what its last boot was asked: its network, and the folders of
	// its Mac shared into it (SC-06: station never shares one).
	Booted  *Boot  `json:"booted,omitempty"`
	Address string `json:"address,omitempty"`
	// value: its guest, answering over the one way into a VM; nil when no
	// guest answers.
	Guest *Guest `json:"guest,omitempty"`
	// value: what listens inside the VM.
	Listening []ports.Listener `json:"listening,omitempty"`
	// value: the containers Docker runs inside the VM, each with the ports
	// it publishes.
	Containers []Container `json:"containers,omitempty"`
}

type Container struct {
	Name      string `json:"name"`
	Published []int  `json:"published"`
}

type Boot struct {
	Bridged bool     `json:"bridged"`
	Folders []string `json:"folders"`
}

type Kept struct {
	Image     string            `json:"image,omitempty"`
	Version   string            `json:"version,omitempty"`
	Lifetime  machines.Lifetime `json:"lifetime"`
	Want      string            `json:"want,omitempty"`
	Network   string            `json:"network,omitempty"`
	Check     string            `json:"check,omitempty"`
	Env       map[string]string `json:"env,omitempty"`
	CreatedAt time.Time         `json:"created_at,omitzero"`
	// value: the shares station v2 kept in the VM's record, by name, until
	// the accounts' store moves them out once.
	Lent []string `json:"lent,omitempty"`
}

func (v *VM) Running() bool { return v.PID != 0 }

func (v *VM) Down(now time.Time, end *machines.RunEnd) {
	if v.Running() {
		v.DownAt = now
	}
	v.PID, v.Ended = 0, end
}

type Guest struct {
	// value: what the guest answers to each command line it is given; a
	// line ending in * answers every line it begins.
	Answers map[string]Answer `json:"answers,omitempty"`
	// value: the VM's own tooling replaced station's guest: its ssh answers,
	// station's session does not.
	Replaced bool `json:"replaced,omitempty"`
	// value: made from station's image by a create: a command no answer
	// scripts succeeds, as the image's first boot does.
	Fresh bool `json:"fresh,omitempty"`
	// value: every command line station ran in the VM, apart from its own
	// session and the liveness probe, in the order run: what it set up inside.
	Ran []string `json:"ran,omitempty"`
}

type Answer struct {
	Code   int    `json:"code,omitempty"`
	Stdout string `json:"stdout,omitempty"`
	Stderr string `json:"stderr,omitempty"`
	// value: the line powers the VM off once it is answered.
	Poweroff bool `json:"poweroff,omitempty"`
}

type Job struct {
	background.Job
	// value: the definition as it was written, so writing it again is seen.
	Definition string `json:"definition,omitempty"`
}

func (w *World) Lock()   { w.mu.Lock() }
func (w *World) Unlock() { w.mu.Unlock() }

func Decode(data []byte) (*World, error) {
	w := &World{}
	if err := json.Unmarshal(data, w); err != nil {
		return nil, err
	}
	return w, nil
}

func (w *World) Encode() ([]byte, error) {
	data, err := json.MarshalIndent(w, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func (w *World) UseHypervisor(hypervisor string) {
	// invariant: AR-02: the same world over another adapter. Every Mac that
	// has the hypervisor station is built with gets the one named instead;
	// a Mac with none installed, or one named already, keeps its own.
	if hypervisor == "" {
		return
	}
	for _, m := range w.Macs {
		if m.Hypervisor == "" {
			m.Hypervisor = hypervisor
		}
	}
}

func (m *Mac) Reads(f catalog.Fact, value string) {
	// value: the Mac answers fact f with value from now on.
	if m.Facts == nil {
		m.Facts = map[catalog.Fact]settings.Fact{}
	}
	m.Facts[f] = settings.Fact{State: catalog.StateOK, Value: value}
}

func (w *World) Mac(name string) *Mac {
	// invariant: Mac answers the Mac of that name, made empty the first time it
	// is asked, so a fake writes its slice without checking it exists.
	if w.Macs == nil {
		w.Macs = map[string]*Mac{}
	}
	m := w.Macs[name]
	if m == nil {
		m = &Mac{}
		w.Macs[name] = m
	}
	if m.VMs == nil {
		m.VMs = map[string]*VM{}
	}
	return m
}

func (w *World) Fails(call string) bool {
	// invariant: a scripted fault fails its call and is spent; one with no
	// count fails every call.
	for i, f := range w.Script {
		if f.Call != call || f.Hold {
			continue
		}
		switch f.Times {
		case 0:
		case 1:
			w.Script = append(w.Script[:i], w.Script[i+1:]...)
		default:
			w.Script[i].Times--
		}
		return true
	}
	return false
}

func (w *World) Holds(call string) bool {
	// invariant: a scripted hold is spent by the one call it holds.
	for i, f := range w.Script {
		if f.Call == call && f.Hold {
			w.Script = append(w.Script[:i], w.Script[i+1:]...)
			return true
		}
	}
	return false
}

func (w *World) Holder(vm string) string {
	// value: the Mac that holds vm, empty when none does.
	for name, m := range w.Macs {
		if _, ok := m.VMs[vm]; ok {
			return name
		}
	}
	return ""
}
