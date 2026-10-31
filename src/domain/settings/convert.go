package settings

import (
	"encoding/json"
	"strings"
)

type v1Document struct {
	Remote struct {
		Mode       string `json:"mode"`
		AuthMethod string `json:"auth_method"`
		Secret     string `json:"secret"`
		Hostname   string `json:"hostname"`
	} `json:"remote"`
	UserEmail string `json:"user_email"`
	UserName  string `json:"user_name"`
	Self      string `json:"self"`
	Machines  map[string]struct {
		SSH      string `json:"ssh"`
		Identity string `json:"identity"`
	} `json:"machines"`
}

func ConvertV1(v1 []byte, user string) (s Settings, authKey string, err error) {
	var old v1Document
	if err := json.Unmarshal(v1, &old); err != nil {
		return Settings{}, "", &UnreadableError{Path: "config.json", Cause: err}
	}
	s = Defaults()
	s.Here = old.Self
	s.VMUser = user
	s.UserEmail = old.UserEmail
	s.UserName = old.UserName
	if old.Remote.Mode == string(ModeAuto) {
		s.Tailnet.Mode = ModeAuto
	}
	if old.Remote.AuthMethod == string(AuthAuthKey) {
		s.Tailnet.AuthMethod = AuthAuthKey
		authKey = old.Remote.Secret
	}
	for name, m := range old.Machines {
		if s.Hosts == nil {
			s.Hosts = map[string]Host{}
		}
		login, _, found := strings.Cut(m.SSH, "@")
		if !found {
			login = ""
		}
		s.Hosts[name] = Host{User: login, Key: m.Identity}
	}
	if s.Here == "" && old.Remote.Hostname != "" {
		s.Here = old.Remote.Hostname
	}
	if s.Here != "" {
		if s.Hosts == nil {
			s.Hosts = map[string]Host{}
		}
		if _, ok := s.Hosts[s.Here]; !ok {
			s.Hosts[s.Here] = Host{}
		}
	}
	return s, authKey, nil
}
