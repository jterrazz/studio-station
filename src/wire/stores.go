package wire

import (
	"path/filepath"

	accountsjsonfile "github.com/jterrazz/studio-station/src/accounts/adapter/jsonfile"
	accountscontract "github.com/jterrazz/studio-station/src/accounts/contract"
	corejsonfile "github.com/jterrazz/studio-station/src/core/adapter/jsonfile"
	"github.com/jterrazz/studio-station/src/core/store"
	"github.com/jterrazz/studio-station/src/domain/settings"
	imagesjsonfile "github.com/jterrazz/studio-station/src/images/adapter/jsonfile"
	machinesjsonfile "github.com/jterrazz/studio-station/src/machines/adapter/jsonfile"
	machinescontract "github.com/jterrazz/studio-station/src/machines/contract"
	portsjsonfile "github.com/jterrazz/studio-station/src/ports/adapter/jsonfile"
	portscontract "github.com/jterrazz/studio-station/src/ports/contract"
	settingsjsonfile "github.com/jterrazz/studio-station/src/settings/adapter/jsonfile"
	setupcontract "github.com/jterrazz/studio-station/src/setup/contract"
	"github.com/jterrazz/studio-station/src/system"
)

// invariant: the preference groups in the order settings.json writes them,
// each group named by its one owner's contract.
func preferenceGroups() []string {
	return []string{
		setupcontract.GroupName, machinescontract.GroupMacs, machinescontract.GroupVMUser,
		accountscontract.GroupShareList, setupcontract.GroupUserEmail, setupcontract.GroupUserName,
		portscontract.GroupPorts,
	}
}

// value: every store station keeps on this machine, each built once here
// over its file; nothing else constructs one.
type stores struct {
	prefs        *corejsonfile.Preferences
	machines     *machinesjsonfile.Store
	images       *imagesjsonfile.Store
	shares       *accountsjsonfile.Shares
	reservations *portsjsonfile.Reservations
	declarations *portsjsonfile.Declarations
	applied      *settingsjsonfile.Applied
}

func newStores(files system.FS, p settings.Paths) *stores {
	return &stores{
		prefs:        corejsonfile.NewPreferences(files, p.Config, preferenceGroups()),
		machines:     machinesjsonfile.New(files, filepath.Join(p.State, "vms")),
		images:       imagesjsonfile.New(files, filepath.Join(p.State, "images")),
		shares:       accountsjsonfile.NewShares(files, filepath.Join(p.State, "vms")),
		reservations: portsjsonfile.NewReservations(files, filepath.Join(p.State, "leases.json")),
		declarations: portsjsonfile.NewDeclarations(files, filepath.Join(p.Config, "public.json")),
		applied:      settingsjsonfile.NewApplied(files, filepath.Join(p.State, "applied")),
	}
}

func (s *stores) migrators() []store.Migrator {
	return []store.Migrator{s.prefs, s.machines, s.images, s.shares, s.reservations, s.declarations, s.applied}
}

func (s *stores) name() setupcontract.NamePrefs         { return setupcontract.Name(s.prefs) }
func (s *stores) identity() setupcontract.IdentityPrefs { return setupcontract.Identity(s.prefs) }
func (s *stores) hosts() machinescontract.MacsPrefs     { return machinescontract.Macs(s.prefs) }
func (s *stores) vmUser() machinescontract.VMUserPrefs  { return machinescontract.VMUser(s.prefs) }
func (s *stores) shareList() accountscontract.ShareListPrefs {
	return accountscontract.ShareList(s.prefs)
}
func (s *stores) services() portscontract.PortsPrefs { return portscontract.Ports(s.prefs) }
