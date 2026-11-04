package calls

import (
	"context"
	"time"

	activitycontract "github.com/jterrazz/studio-station/src/activity/contract"
	backgroundcontract "github.com/jterrazz/studio-station/src/background/contract"
	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/core/call/lanes"
	"github.com/jterrazz/studio-station/src/overview"
	"github.com/jterrazz/studio-station/src/ports"
	portscontract "github.com/jterrazz/studio-station/src/ports/contract"
	settingscontract "github.com/jterrazz/studio-station/src/settings/contract"
	setupcontract "github.com/jterrazz/studio-station/src/setup/contract"
)

const ReadBudget = defaultBudget

type Here struct {
	Kind catalog.MachineKind
	// value: what the process knows of this machine without asking it.
	Hostname, OS, Arch, User, Shell string
	// value: the owner's home and the system root, so a path reads as the owner types it.
	Home, Root      string
	Version, Commit string
	// value: whether the daemon's job definition is on disk.
	Agent func() bool
}

type Clock interface{ Now() time.Time }

type Deps[Tool, Item, Stray any] struct {
	// value: the reads Overview composes. Tool, Item and Stray are the rows of
	// the Apps, Settings and Disk reads, until those reads are their families' calls.
	Pool  *lanes.Pool
	Clock Clock
	Here  Here
	Name  setupcontract.NamePrefs
	// value: the git identity the owner's preferences expect.
	Identity setupcontract.IdentityPrefs
	// value: the facts of this machine (its kernel, network, protections and
	// updates), and those of its owner (git, gpg, GitHub, the ssh key).
	Facts, Owner settingscontract.Inspector
	Meter        activitycontract.Meter
	Listeners    portscontract.Listeners
	Jobs         backgroundcontract.Jobs
	// value: the app updates waiting, on a Mac; nil elsewhere.
	Updates    Updates
	Tailnet    Tailnet
	Daemon     Daemon
	Power      Power
	Hypervisor Hypervisor
	Tools      Tools[Tool]
	Config     Config[Item]
	Setup      Setup
	Disk       Disk[Stray]
}

type Updates interface {
	// value: the manager's name, and how many of its installs have a newer
	// version; apps contract.ErrMissing where the manager is not installed.
	Manager() string
	Behind(ctx context.Context) (int, error)
}

type Tailnet interface {
	Status(ctx context.Context) (ports.Status, error)
}

type Daemon interface {
	Ping(ctx context.Context) (version string, err error)
}

type Power interface {
	// value: the daemon's Power session on this host; ok false where Power
	// cannot be read (a VM, no power management, the daemon down).
	Session(ctx context.Context) (s overview.PowerState, ok bool)
}

type Hypervisor interface {
	Name() string
	Installed() bool
}

type Tools[Tool any] interface {
	Inventory(ctx context.Context) ([]Tool, error)
}

type Config[Item any] interface {
	List(ctx context.Context) ([]Item, error)
	// value: the config item a row is, and its state.
	Placed(r Item) (item string, state catalog.State)
}

type Setup interface {
	Check(ctx context.Context) ([]overview.Step, error)
}

type DiskReading[Stray any] struct {
	Rows   []overview.Disk
	Strays []Stray
}

type Disk[Stray any] interface {
	// invariant: each reading is the whole disk read so far; the last is final.
	Watch(ctx context.Context) <-chan DiskReading[Stray]
}
