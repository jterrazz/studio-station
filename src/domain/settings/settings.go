package settings

import (
	"encoding/json"
	"errors"
	"io/fs"
	"path"

	"github.com/jterrazz/studio-station/src/system"
)

type Paths struct{ Config, State, Cache, Bin string }

type Mode string

const (
	ModeAuto      Mode = "auto"
	ModeUserspace Mode = "userspace"
)

type AuthMethod string

const (
	AuthLogin   AuthMethod = "login"
	AuthAuthKey AuthMethod = "authkey"
)

type Host struct {
	User string `json:"user,omitempty"`
	Key  string `json:"key,omitempty"`
}

type Tailnet struct {
	Mode       Mode       `json:"mode"`
	AuthMethod AuthMethod `json:"auth_method"`
}

type Settings struct {
	Here      string          `json:"here,omitempty"`
	Hosts     map[string]Host `json:"hosts,omitempty"`
	Tailnet   Tailnet         `json:"tailnet"`
	VMUser    string          `json:"vm_user,omitempty"`
	LendList  []string        `json:"lend_list,omitempty"`
	UserEmail string          `json:"user_email,omitempty"`
	UserName  string          `json:"user_name,omitempty"`
}

const (
	defaultEmail = "admin@jterrazz.com"
	defaultName  = "Jean-Baptiste Terrazzoni"
	fileName     = "settings.json"
)

func (v Settings) Email() string {
	if v.UserEmail != "" {
		return v.UserEmail
	}
	return defaultEmail
}

func (v Settings) Name() string {
	if v.UserName != "" {
		return v.UserName
	}
	return defaultName
}

func Defaults() Settings {
	return Settings{Tailnet: Tailnet{Mode: ModeUserspace, AuthMethod: AuthLogin}}
}

type Store struct {
	fs   system.FS
	dir  string
	path string
}

func NewStore(files system.FS, p Paths) Store {
	return Store{fs: files, dir: p.Config, path: path.Join(p.Config, fileName)}
}

func (s Store) Load() (Settings, error) {
	v := Defaults()
	data, err := s.fs.ReadFile(s.path)
	if errors.Is(err, fs.ErrNotExist) {
		return v, nil
	}
	if err != nil {
		return v, &UnreadableError{Path: s.path, Cause: err}
	}
	if err := json.Unmarshal(data, &v); err != nil {
		return Defaults(), &UnreadableError{Path: s.path, Cause: err}
	}
	if v.Tailnet.Mode == "" {
		v.Tailnet.Mode = ModeUserspace
	}
	if v.Tailnet.AuthMethod == "" {
		v.Tailnet.AuthMethod = AuthLogin
	}
	return v, nil
}

func (s Store) Save(v Settings) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	if err := s.fs.MkdirAll(s.dir, 0o700); err != nil {
		return &UnwritableError{Path: s.path, Cause: err}
	}
	if err := s.fs.WriteFile(s.path, append(data, '\n'), 0o600); err != nil {
		return &UnwritableError{Path: s.path, Cause: err}
	}
	return nil
}

type UnreadableError struct {
	Path  string
	Cause error
}

func (e *UnreadableError) Error() string { return "cannot read " + e.Path + ": " + e.Cause.Error() }
func (e *UnreadableError) Unwrap() error { return e.Cause }

type UnwritableError struct {
	Path  string
	Cause error
}

func (e *UnwritableError) Error() string { return "cannot write " + e.Path + ": " + e.Cause.Error() }
func (e *UnwritableError) Unwrap() error { return e.Cause }
