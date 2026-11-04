package contract

import (
	"github.com/jterrazz/studio-station/src/accounts"
	core "github.com/jterrazz/studio-station/src/core/contract"
)

const (
	GroupShareList = "share_list"
	// value: the key station v2 kept the list under, read while no
	// share_list is written.
	GroupLendList = "lend_list"
)

type ShareListPrefs struct{ p core.Preferences }

func ShareList(p core.Preferences) ShareListPrefs { return ShareListPrefs{p: p} }

func (s ShareListPrefs) Get() ([]string, error) {
	// invariant: the list is the owner's, written by hand; a Mac whose
	// preferences name none shares accounts.DefaultShareList.
	for _, group := range []string{GroupShareList, GroupLendList} {
		var names []string
		if err := s.p.Read(group, &names); err != nil {
			return nil, err
		}
		if names != nil {
			return names, nil
		}
	}
	return accounts.DefaultShareList(), nil
}

func (s ShareListPrefs) Set(names []string) error {
	// invariant: the list is written under its v3 key; the v2 key goes.
	if err := s.p.Write(GroupShareList, names); err != nil {
		return err
	}
	return s.p.Write(GroupLendList, nil)
}
