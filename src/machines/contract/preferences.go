package contract

import (
	core "github.com/jterrazz/studio-station/src/core/contract"
	"github.com/jterrazz/studio-station/src/machines"
)

const (
	GroupMacs   = "hosts"
	GroupVMUser = "vm_user"
)

type MacsPrefs struct{ p core.Preferences }

func Macs(p core.Preferences) MacsPrefs { return MacsPrefs{p: p} }

func (h MacsPrefs) Get() (map[string]machines.Host, error) {
	var hosts map[string]machines.Host
	err := h.p.Read(GroupMacs, &hosts)
	return hosts, err
}

func (h MacsPrefs) Set(hosts map[string]machines.Host) error { return h.p.Write(GroupMacs, hosts) }

type VMUserPrefs struct{ p core.Preferences }

func VMUser(p core.Preferences) VMUserPrefs { return VMUserPrefs{p: p} }

func (v VMUserPrefs) Get() (string, error) {
	var user string
	err := v.p.Read(GroupVMUser, &user)
	return user, err
}

func (v VMUserPrefs) Set(user string) error { return v.p.Write(GroupVMUser, user) }
