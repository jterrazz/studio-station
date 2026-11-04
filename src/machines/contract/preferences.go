package contract

import (
	core "github.com/jterrazz/studio-station/src/core/contract"
	"github.com/jterrazz/studio-station/src/machines"
)

const (
	GroupHosts  = "hosts"
	GroupVMUser = "vm_user"
)

type HostsPrefs struct{ p core.Preferences }

func Hosts(p core.Preferences) HostsPrefs { return HostsPrefs{p: p} }

func (h HostsPrefs) Get() (map[string]machines.Host, error) {
	var hosts map[string]machines.Host
	err := h.p.Read(GroupHosts, &hosts)
	return hosts, err
}

func (h HostsPrefs) Set(hosts map[string]machines.Host) error { return h.p.Write(GroupHosts, hosts) }

type VMUserPrefs struct{ p core.Preferences }

func VMUser(p core.Preferences) VMUserPrefs { return VMUserPrefs{p: p} }

func (v VMUserPrefs) Get() (string, error) {
	var user string
	err := v.p.Read(GroupVMUser, &user)
	return user, err
}

func (v VMUserPrefs) Set(user string) error { return v.p.Write(GroupVMUser, user) }
