package tailnet

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/system"
)

const queryBudget = 5 * time.Second

type Service struct {
	h    system.Host
	owns func(ServeEntry) bool
}

func New(h system.Host, owns func(ServeEntry) bool) *Service { return &Service{h: h, owns: owns} }

type Peer struct {
	ID, Name, IP string
	Online       bool
	Tags         []string
}

type Self struct {
	Name, DNSName, IP, BackendState string
}

type ServeEntry struct {
	Port    int
	Path    string
	Target  string
	Station bool
}

type ServeConfig struct{ Entries []ServeEntry }

type Status struct {
	State        catalog.State
	Connected    bool
	BackendState string
	Name, IP     string
	DNSName      string
	Peers        []Peer
	Serve        []ServeEntry
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

var ErrAbsent = errors.New("tailscale is not installed")

func (n *rawNode) name() string {
	if n.HostName != "" && n.HostName != "localhost" {
		return n.HostName
	}
	label, _, _ := strings.Cut(strings.TrimSuffix(n.DNSName, "."), ".")
	return label
}

func (n *rawNode) ip() string {
	for _, ip := range n.TailscaleIPs {
		if !strings.Contains(ip, ":") {
			return ip
		}
	}
	if len(n.TailscaleIPs) == 0 {
		return ""
	}
	return n.TailscaleIPs[0]
}

func (s *Service) run(ctx context.Context, args ...string) (system.Output, error) {
	if _, ok := s.h.Run.LookPath("tailscale"); !ok {
		return system.Output{}, ErrAbsent
	}
	return s.h.Run.Output(ctx, system.Cmd{Name: "tailscale", Args: args, Timeout: queryBudget})
}

func (s *Service) query(ctx context.Context) (rawStatus, error) {
	var st rawStatus
	out, err := s.run(ctx, "status", "--json")
	if err != nil {
		return st, err
	}
	if err := json.Unmarshal([]byte(out.Stdout), &st); err != nil {
		if out.ExitCode != 0 {
			return st, fmt.Errorf("tailscale status exited %d", out.ExitCode)
		}
		return st, fmt.Errorf("tailscale status: %w", err)
	}
	return st, nil
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

func (s *Service) Roster(ctx context.Context) ([]Peer, error) {
	st, err := s.query(ctx)
	if err != nil {
		return nil, err
	}
	roster := peersOf(st)
	if st.Self != nil {
		roster = append([]Peer{{ID: st.Self.ID, Name: st.Self.name(), IP: st.Self.ip(), Online: st.BackendState == "Running", Tags: st.Self.Tags}}, roster...)
	}
	return roster, nil
}

func (s *Service) Self(ctx context.Context) (Self, error) {
	st, err := s.query(ctx)
	if err != nil {
		return Self{}, err
	}
	out := Self{BackendState: st.BackendState}
	if st.Self != nil {
		out.Name, out.IP = st.Self.name(), st.Self.ip()
		out.DNSName = strings.TrimSuffix(st.Self.DNSName, ".")
	}
	return out, nil
}

func (s *Service) Status(ctx context.Context) (Status, error) {
	// invariant: station reads the app and drives only `tailscale serve`; an app
	// that is down or absent is a state, never an error.
	out := Status{State: catalog.StateAbsent}
	st, err := s.query(ctx)
	if err != nil {
		if !errors.Is(err, ErrAbsent) {
			out.State = catalog.StateDegraded
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
		out.DNSName = strings.TrimSuffix(st.Self.DNSName, ".")
	}
	out.Peers = peersOf(st)
	if serve, err := s.ServeStatus(ctx); err == nil {
		for _, e := range serve.Entries {
			e.Station = s.owns != nil && s.owns(e)
			out.Serve = append(out.Serve, e)
		}
	}
	return out, nil
}

func (s *Service) Serve(ctx context.Context, args ...string) error {
	// invariant: a serve that never returns (the app waits for the admin to
	// enable Serve) is refused with the line it printed, not with the timeout.
	out, err := s.run(ctx, append([]string{"serve"}, args...)...)
	if errors.Is(err, system.ErrTimeout) {
		if detail := reason(out.Stderr + "\n" + out.Stdout); detail != "" {
			return &ServeError{Detail: detail}
		}
	}
	if err != nil {
		return err
	}
	if out.ExitCode != 0 {
		detail := reason(out.Stderr + "\n" + out.Stdout)
		if detail == "" {
			detail = "exit " + strconv.Itoa(out.ExitCode)
		}
		return &ServeError{Detail: detail}
	}
	return nil
}

type rawServe struct {
	TCP map[string]struct {
		HTTPS bool `json:"HTTPS"`
	} `json:"TCP"`
	Web map[string]struct {
		Handlers map[string]struct {
			Proxy string `json:"Proxy"`
		} `json:"Handlers"`
	} `json:"Web"`
}

func (s *Service) ServeStatus(ctx context.Context) (ServeConfig, error) {
	out, err := s.run(ctx, "serve", "status", "--json")
	if err != nil {
		return ServeConfig{}, err
	}
	if out.ExitCode != 0 {
		return ServeConfig{}, &ServeError{Detail: firstLine(out.Stderr)}
	}
	return ParseServe([]byte(out.Stdout))
}

func ParseServe(data []byte) (ServeConfig, error) {
	var raw rawServe
	if strings.TrimSpace(string(data)) == "" {
		return ServeConfig{}, nil
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return ServeConfig{}, err
	}
	var cfg ServeConfig
	seen := map[int]bool{}
	for hostPort, web := range raw.Web {
		_, portText, _ := strings.Cut(hostPort, ":")
		port, _ := strconv.Atoi(portText)
		for p, h := range web.Handlers {
			cfg.Entries = append(cfg.Entries, ServeEntry{Port: port, Path: p, Target: h.Proxy})
			seen[port] = true
		}
	}
	for portText := range raw.TCP {
		port, err := strconv.Atoi(portText)
		if err == nil && !seen[port] {
			cfg.Entries = append(cfg.Entries, ServeEntry{Port: port})
		}
	}
	sort.Slice(cfg.Entries, func(i, j int) bool {
		if cfg.Entries[i].Port != cfg.Entries[j].Port {
			return cfg.Entries[i].Port < cfg.Entries[j].Port
		}
		return cfg.Entries[i].Path < cfg.Entries[j].Path
	})
	return cfg, nil
}

// invariant: a refusal is told by its own line: the CLI's "Warning:" lines (a
// client older than its daemon) come first and say nothing about the serve.
func reason(text string) string {
	for _, line := range strings.Split(text, "\n") {
		if line = strings.TrimSpace(line); line != "" && !strings.HasPrefix(line, "Warning:") {
			return line
		}
	}
	return firstLine(text)
}

func firstLine(text string) string {
	line, _, _ := strings.Cut(strings.TrimSpace(text), "\n")
	return strings.TrimSpace(line)
}
