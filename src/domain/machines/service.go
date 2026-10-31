package machines

import (
	"context"
	"path"
	"strings"
	"time"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/domain/settings"
	"github.com/jterrazz/studio-station/src/system"
)

type Ports struct{ OpenClaw, Hermes int }

type Options struct {
	Ports   Ports
	Here    catalog.MachineKind
	Paths   settings.Paths
	User    string
	Version string
	Remote  func(target string, words []string) []string
}

type Node struct {
	Online bool
	IP     string
}

type Tailnet interface {
	Mint(ctx context.Context) (secretText string, err error)
	Remove(ctx context.Context, nodeID string) error
	Peers(ctx context.Context) (map[string]Node, error)
	Argv(args ...string) []string
	Mode() string
}

type Images interface {
	Latest(ctx context.Context, image string) (tartName, version string, err error)
}

type Service struct {
	h      system.Host
	engine system.Engine
	store  settings.Store
	tn     Tailnet
	img    Images
	o      Options
}

func New(h system.Host, e system.Engine, store settings.Store, tn Tailnet, img Images, o Options) *Service {
	return &Service{h: h, engine: e, store: store, tn: tn, img: img, o: o}
}

type Lifetime string

const (
	Temporary Lifetime = "temporary"
	Lasting   Lifetime = "lasting"
)

type VMState string

const (
	Running   VMState = "running"
	Stopped   VMState = "stopped"
	Suspended VMState = "suspended"
	Stale     VMState = "stale"
)

type VMRecord struct {
	Name, Image, Version, NodeID string
	Lifetime                     Lifetime
	Resources                    system.Resources
	Lent                         []string
	CreatedAt                    time.Time
}

type Machine struct {
	Name      string
	Kind      catalog.MachineKind
	Here      bool
	Host      string
	User, Key string
	Lifetime  Lifetime
}

type VM struct {
	Name, Image, Version, Host string
	Kind                       catalog.MachineKind
	Lifetime                   Lifetime
	State                      VMState
	Resources                  system.Resources
	SizeGB                     int
	Node                       Node
	Lent                       []string
	CreatedAt                  time.Time
}

type HostView struct {
	Name, User, StationVersion string
	Here, Reachable            bool
	Node                       Node
	VMs                        []VM
}

type Fleet struct{ Hosts []HostView }

type Check struct {
	Name          string
	State         catalog.State
	Value, Detail string
}

type ProbeReport struct {
	Name, User, IP string
	Checks         []Check
	Console        string
}

func (s *Service) vmUser(cfg settings.Settings) string {
	if cfg.VMUser != "" {
		return cfg.VMUser
	}
	return s.o.User
}

func (s *Service) recordsDir() string { return path.Join(s.o.Paths.State, "vms") }

func (s *Service) knownHostsDir() string { return path.Join(s.o.Paths.State, "known_hosts") }

func (s *Service) sshConfig() string { return path.Join(s.h.Env.Home(), ".ssh", "config") }

func (s *Service) CheckHost() error {
	if s.o.Here != catalog.Host {
		return &OnlyOnHostError{Here: s.o.Here}
	}
	return nil
}

var nameChars = func(r rune) bool { return r == '-' || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') }

func validName(name string) bool {
	return len(name) >= 1 && len(name) <= 40 && strings.IndexFunc(name, func(r rune) bool { return !nameChars(r) }) < 0
}
