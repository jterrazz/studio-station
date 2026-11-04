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
}

type Fault struct {
	// value: the method that fails, "<Contract>.<Method>": "Installer.Install".
	Call string `json:"call"`
	// value: how many more calls fail; zero fails every call.
	Times int `json:"times,omitempty"`
}

type Mac struct {
	// value: the engine of this Mac; nil when none was ever set up.
	Engine *Engine `json:"engine,omitempty"`
	// value: FileVault waits for its password at pre-boot: nothing of the
	// Mac runs, its engine neither.
	Locked bool `json:"locked,omitempty"`
	// value: out of the tailnet's reach.
	Offline bool `json:"offline,omitempty"`
	// value: what station keeps in this Mac's preferences, by group.
	Preferences map[string]json.RawMessage `json:"preferences,omitempty"`
	// value: station's own files as text, by name, while no store has read
	// them: a file edited by hand, a file station v2 left.
	Files map[string]string `json:"files,omitempty"`
	// value: the images the hypervisor holds, by reference.
	Images []string       `json:"images,omitempty"`
	VMs    map[string]*VM `json:"vms,omitempty"`

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
}

type Node struct {
	Self   ports.Self    `json:"self"`
	Shares []ports.Share `json:"shares,omitempty"`
}

type Manager struct {
	// value: the manager itself is not installed.
	Absent    bool              `json:"absent,omitempty"`
	Installed map[string]string `json:"installed,omitempty"`
	// value: the keys a newer version waits for.
	Behind []string `json:"behind,omitempty"`
}

type Meter struct {
	Sample activity.Sample `json:"sample"`
	Uptime time.Duration   `json:"uptime,omitempty"`
	// value: the battery's line ("" for none) and the memory installed.
	Battery string `json:"battery,omitempty"`
	Memory  int64  `json:"memory,omitempty"`
}

type Disk struct {
	Total int64            `json:"total,omitempty"`
	Free  int64            `json:"free,omitempty"`
	Files map[string]int64 `json:"files,omitempty"`
	// value: the processes that run, by name.
	Running []string `json:"running,omitempty"`
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
	Ended   *machines.RunEnd `json:"ended,omitempty"`
	Address string           `json:"address,omitempty"`
	// value: its guest, answering over the one way into a VM; nil when no
	// guest answers.
	Guest *Guest `json:"guest,omitempty"`
	// value: what listens inside the VM.
	Listening []ports.Listener `json:"listening,omitempty"`
}

type Kept struct {
	Image     string            `json:"image,omitempty"`
	Version   string            `json:"version,omitempty"`
	Lifetime  machines.Lifetime `json:"lifetime"`
	Want      string            `json:"want,omitempty"`
	Network   string            `json:"network,omitempty"`
	Env       map[string]string `json:"env,omitempty"`
	CreatedAt time.Time         `json:"created_at,omitzero"`
	Shares    []string          `json:"shares,omitempty"`
}

func (v *VM) Running() bool { return v.PID != 0 }

type Guest struct {
	// value: what the guest answers to each command line it is given.
	Answers map[string]Answer `json:"answers,omitempty"`
}

type Answer struct {
	Code   int    `json:"code,omitempty"`
	Stdout string `json:"stdout,omitempty"`
	Stderr string `json:"stderr,omitempty"`
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
		if f.Call != call {
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

func (w *World) Holder(vm string) string {
	// value: the Mac that holds vm, empty when none does.
	for name, m := range w.Macs {
		if _, ok := m.VMs[vm]; ok {
			return name
		}
	}
	return ""
}
