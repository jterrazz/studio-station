package settings

import (
	"encoding/json"
	"strings"

	"github.com/jterrazz/studio-station/src/core/store"
)

type v1Document struct {
	Remote struct {
		Hostname string `json:"hostname"`
	} `json:"remote"`
	UserEmail string `json:"user_email"`
	UserName  string `json:"user_name"`
	Self      string `json:"self"`
	Machines  map[string]struct {
		SSH      string `json:"ssh"`
		Identity string `json:"identity"`
	} `json:"machines"`
}

func ConvertV1(v1 []byte, user string) (Settings, error) {
	// invariant: the v1 tailnet mode and auth key are dropped: the Tailscale app
	// logs in itself, and station keeps no tailnet secret.
	var old v1Document
	if err := json.Unmarshal(v1, &old); err != nil {
		return Settings{}, &store.UnreadableError{Path: "config.json", Cause: err}
	}
	var s Settings
	s.Here = old.Self
	s.VMUser = user
	s.UserEmail = old.UserEmail
	s.UserName = old.UserName
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
	return s, nil
}
