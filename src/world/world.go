package world

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/jterrazz/studio-station/src/accounts"
	"github.com/jterrazz/studio-station/src/background"
	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/engine"
	"github.com/jterrazz/studio-station/src/machines"
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
}

type Engine struct {
	Version string `json:"version,omitempty"`
	// value: set up, and stopped by the owner.
	Stopped bool `json:"stopped,omitempty"`
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

func (w *World) Holder(vm string) string {
	// value: the Mac that holds vm, empty when none does.
	for name, m := range w.Macs {
		if _, ok := m.VMs[vm]; ok {
			return name
		}
	}
	return ""
}
