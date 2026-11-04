package contract

import core "github.com/jterrazz/studio-station/src/core/contract"

const GroupShareList = "lend_list"

type ShareListPrefs struct{ p core.Preferences }

func ShareList(p core.Preferences) ShareListPrefs { return ShareListPrefs{p: p} }

func (s ShareListPrefs) Get() ([]string, error) {
	var names []string
	err := s.p.Read(GroupShareList, &names)
	return names, err
}

func (s ShareListPrefs) Set(names []string) error { return s.p.Write(GroupShareList, names) }
