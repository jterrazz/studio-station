package tailnet

import (
	"path"

	"github.com/jterrazz/studio-station/src/domain/settings"
	"github.com/jterrazz/studio-station/src/system"
)

const (
	hostAuthKeyAccount = "tailnet-authkey"
	joinIDAccount      = "vm-join-id"
	joinSecretAccount  = "vm-join-secret"
)

type Options struct {
	Paths       settings.Paths
	APIURL      string
	Interactive bool
}

type Service struct {
	h     system.Host
	store settings.Store
	keys  settings.Keychain
	o     Options
}

func New(h system.Host, store settings.Store, keys settings.Keychain, o Options) *Service {
	return &Service{h: h, store: store, keys: keys, o: o}
}

func (s *Service) dir() string       { return path.Join(s.o.Paths.State, "tailscaled") }
func (s *Service) socket() string    { return path.Join(s.dir(), "tailscaled.sock") }
func (s *Service) stateFile() string { return path.Join(s.dir(), "tailscaled.state") }
func (s *Service) logFile() string   { return path.Join(s.dir(), "tailscaled.log") }
func (s *Service) pidFile() string   { return path.Join(s.dir(), "tailscaled.pid") }
func (s *Service) awakeFile() string { return path.Join(s.dir(), "caffeinate.pid") }

func (s *Service) mode() settings.Mode {
	cfg, err := s.store.Load()
	if err != nil {
		return settings.ModeUserspace
	}
	return cfg.Tailnet.Mode
}

// Argv answers the arguments of a tailscale call that talks to the daemon the
// ssh blocks and the fleet use: station's own socket in userspace mode.
func (s *Service) Argv(args ...string) []string {
	if s.mode() == settings.ModeUserspace {
		return s.daemonArgv(Userspace, args...)
	}
	return args
}

func (s *Service) Mode() string { return string(s.mode()) }

func (s *Service) daemonArgv(d Daemon, args ...string) []string {
	if d == Userspace {
		return append([]string{"--socket=" + s.socket()}, args...)
	}
	return args
}
