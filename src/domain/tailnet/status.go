package tailnet

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/domain/settings"
	"github.com/jterrazz/studio-station/src/system"
)

type Daemon string

const (
	Userspace Daemon = "userspace"
	System    Daemon = "system"
)

type Peer struct {
	ID, Name, IP string
	Online       bool
	Tags         []string
}

type Status struct {
	State        catalog.State
	Connected    bool
	BackendState string
	Daemon       Daemon
	Name, IP     string
	KeepAwake    bool
	Peers        []Peer
	Mode         settings.Mode
	AuthMethod   settings.AuthMethod
	VMJoin       catalog.State
}

type rawNode struct {
	ID           string   `json:"ID"`
	HostName     string   `json:"HostName"`
	DNSName      string   `json:"DNSName"`
	TailscaleIPs []string `json:"TailscaleIPs"`
	Online       bool     `json:"Online"`
	Tags         []string `json:"Tags"`
}

type rawStatus struct {
	BackendState string              `json:"BackendState"`
	Self         *rawNode            `json:"Self"`
	Peer         map[string]*rawNode `json:"Peer"`
}

func (n *rawNode) name() string {
	if n.HostName != "" && n.HostName != "localhost" {
		return n.HostName
	}
	label, _, _ := strings.Cut(strings.TrimSuffix(n.DNSName, "."), ".")
	return label
}

func (n *rawNode) ip() string {
	if len(n.TailscaleIPs) == 0 {
		return ""
	}
	return n.TailscaleIPs[0]
}

func (s *Service) query(ctx context.Context, d Daemon) (rawStatus, error) {
	var st rawStatus
	out, err := s.h.Run.Output(ctx, system.Cmd{Name: "tailscale", Args: s.daemonArgv(d, "status", "--json")})
	if err != nil {
		return st, err
	}
	if out.ExitCode != 0 {
		return st, fmt.Errorf("tailscale status exited %d", out.ExitCode)
	}
	if err := json.Unmarshal([]byte(out.Stdout), &st); err != nil {
		return st, fmt.Errorf("tailscale status: %w", err)
	}
	return st, nil
}

func (s *Service) userspaceRunning(ctx context.Context) bool {
	_, err := s.query(ctx, Userspace)
	return err == nil
}

func (s *Service) activeDaemon(ctx context.Context, m settings.Mode) Daemon {
	if m == settings.ModeUserspace || s.userspaceRunning(ctx) {
		return Userspace
	}
	return System
}

func peersOf(st rawStatus) []Peer {
	peers := make([]Peer, 0, len(st.Peer))
	for _, n := range st.Peer {
		if n == nil {
			continue
		}
		name := n.name()
		if name == "" {
			name = "unknown"
		}
		peers = append(peers, Peer{ID: n.ID, Name: name, IP: n.ip(), Online: n.Online, Tags: n.Tags})
	}
	sort.Slice(peers, func(i, j int) bool {
		if peers[i].Online != peers[j].Online {
			return peers[i].Online
		}
		return peers[i].Name < peers[j].Name
	})
	return peers
}

func (s *Service) Peers(ctx context.Context) ([]Peer, error) {
	st, err := s.query(ctx, s.activeDaemon(ctx, s.mode()))
	if err != nil {
		return nil, err
	}
	return peersOf(st), nil
}

func (s *Service) Roster(ctx context.Context) ([]Peer, error) {
	st, err := s.query(ctx, s.activeDaemon(ctx, s.mode()))
	if err != nil {
		return nil, err
	}
	roster := peersOf(st)
	if st.Self != nil {
		roster = append([]Peer{{ID: st.Self.ID, Name: st.Self.name(), IP: st.Self.ip(), Online: st.BackendState == "Running", Tags: st.Self.Tags}}, roster...)
	}
	return roster, nil
}

func (s *Service) Status(ctx context.Context) (Status, error) {
	// invariant: a daemon that is down is a state, not an error; only a daemon that
	// cannot be queried while the settings cannot be read fails.
	cfg, cfgErr := s.store.Load()
	daemon := s.activeDaemon(ctx, cfg.Tailnet.Mode)
	out := Status{
		State:      catalog.StateAbsent,
		Daemon:     daemon,
		Mode:       cfg.Tailnet.Mode,
		AuthMethod: cfg.Tailnet.AuthMethod,
		VMJoin:     s.vmJoin(ctx),
	}
	st, err := s.query(ctx, daemon)
	if err != nil {
		if cfgErr != nil {
			return out, cfgErr
		}
		return out, nil
	}
	out.BackendState = st.BackendState
	out.Connected = st.BackendState == "Running"
	out.State = catalog.StateDegraded
	if out.Connected {
		out.State = catalog.StateOK
	}
	if st.Self != nil {
		out.Name, out.IP = st.Self.name(), st.Self.ip()
	}
	out.KeepAwake = daemon == Userspace && s.keepAwake()
	out.Peers = peersOf(st)
	return out, nil
}

func (s *Service) vmJoin(ctx context.Context) catalog.State {
	for _, account := range []string{joinIDAccount, joinSecretAccount} {
		_, found, err := s.keys.Read(ctx, account)
		switch {
		case errors.Is(err, settings.ErrNoKeychain):
			return catalog.StateAbsent
		case err != nil:
			return catalog.StateUnknown
		case !found:
			return catalog.StateAbsent
		}
	}
	return catalog.StateUnknown
}
