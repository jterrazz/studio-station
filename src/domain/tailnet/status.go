package tailnet

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/system"
)

const queryBudget = 5 * time.Second

const (
	VariantStandalone = "standalone"
	VariantAppStore   = "app-store"
)

type Service struct {
	h     system.Host
	root  string
	owns  func(ServeEntry) bool
	track string
}

func New(h system.Host, root string, owns func(ServeEntry) bool) *Service {
	return &Service{h: h, root: root, owns: owns}
}

type CLI struct{ Path, Variant string }

func (s *Service) Variant() string {
	if _, err := s.h.FS.Stat(path.Join("/", s.root, catalog.TailscaleReceipt)); err == nil {
		return VariantAppStore
	}
	if _, err := s.h.FS.Stat(path.Join("/", s.root, catalog.TailscaleApp)); err == nil {
		return VariantStandalone
	}
	return ""
}

func (s *Service) AppStoreCLI() string {
	if s.Variant() != VariantAppStore {
		return ""
	}
	return path.Join("/", s.root, catalog.TailscaleBundleCLI)
}

func (s *Service) CLI() CLI {
	// invariant: the CLI is resolved at each call, by absolute path: a launchd
	// agent's PATH holds neither Homebrew's bin nor ~/.local/bin, and the App
	// Store app has no CLI on any PATH, so the app's own paths come first and
	// PATH last.
	c := CLI{Variant: s.Variant()}
	for _, p := range catalog.TailscaleCLIs {
		if found, ok := s.h.Run.LookPath(path.Join("/", s.root, p)); ok {
			c.Path = found
			return c
		}
	}
	if found, ok := s.h.Run.LookPath("tailscale"); ok {
		c.Path = found
	}
	return c
}

func (s *Service) Tracked(dir string) *Service {
	t := *s
	t.track = dir
	return &t
}

type Peer struct {
	ID, Name, IP string
	Online       bool
	Tags         []string
}

type Self struct {
	Name, DNSName, IP, BackendState string
	ServeOn                         bool
}

func ServeOn(backendState string, certDomains []string) bool {
	// invariant: Serve is on when the tailnet lists a certificate domain: Serve
	// needs HTTPS certificates, and a node logged out or absent serves nothing.
	return backendState == "Running" && len(certDomains) > 0
}

type ServeEntry struct {
	Port    int
	Path    string
	Target  string
	TCP     bool
	Station bool
}

type ServeConfig struct{ Entries []ServeEntry }

type Status struct {
	State        catalog.State
	Connected    bool
	BackendState string
	Name, IP     string
	DNSName      string
	Variant, CLI string
	ServeOn      bool
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
	CertDomains  []string            `json:"CertDomains"`
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

func (s *Service) run(ctx context.Context, budget time.Duration, args ...string) (system.Output, error) {
	cli := s.CLI().Path
	if cli == "" {
		return system.Output{}, ErrAbsent
	}
	out, err := s.h.Run.Output(ctx, system.Cmd{Name: cli, Args: args, Timeout: budget, Track: s.track})
	out.Stdout, out.Stderr = withoutWarnings(out.Stdout), withoutWarnings(out.Stderr)
	return out, err
}

// invariant: a CLI older or newer than its daemon prints "Warning: client
// version ..." before every answer, on either stream: the line says nothing
// about the answer, so nothing reads it.
func withoutWarnings(text string) string {
	if !strings.Contains(text, "Warning:") {
		return text
	}
	lines := strings.SplitAfter(text, "\n")
	kept := lines[:0]
	for _, line := range lines {
		if !strings.HasPrefix(strings.TrimSpace(line), "Warning:") {
			kept = append(kept, line)
		}
	}
	return strings.Join(kept, "")
}

func (s *Service) query(ctx context.Context) (rawStatus, error) {
	var st rawStatus
	out, err := s.run(ctx, queryBudget, "status", "--json")
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
	out := Self{BackendState: st.BackendState, ServeOn: ServeOn(st.BackendState, st.CertDomains)}
	if st.Self != nil {
		out.Name, out.IP = st.Self.name(), st.Self.ip()
		out.DNSName = strings.TrimSuffix(st.Self.DNSName, ".")
	}
	return out, nil
}

func (s *Service) Status(ctx context.Context) (Status, error) {
	// invariant: station reads the app and drives only `tailscale serve` and
	// `tailscale cert`; an app that is down or absent is a state, never an error.
	cli := s.CLI()
	out := Status{State: catalog.StateAbsent, Variant: cli.Variant, CLI: cli.Path}
	st, err := s.query(ctx)
	if err != nil {
		if !errors.Is(err, ErrAbsent) {
			out.State = catalog.StateDegraded
		}
		return out, nil
	}
	out.BackendState = st.BackendState
	out.Connected = st.BackendState == "Running"
	out.ServeOn = ServeOn(st.BackendState, st.CertDomains)
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
	out, err := s.run(ctx, queryBudget, append([]string{"serve"}, args...)...)
	if errors.Is(err, system.ErrTimeout) {
		if detail := firstLine(out.Stderr + "\n" + out.Stdout); detail != "" {
			return &ServeError{Detail: detail}
		}
	}
	if err != nil {
		return err
	}
	if out.ExitCode != 0 {
		detail := firstLine(out.Stderr + "\n" + out.Stdout)
		if detail == "" {
			detail = "exit " + strconv.Itoa(out.ExitCode)
		}
		return &ServeError{Detail: detail}
	}
	return nil
}

func (s *Service) Cert(ctx context.Context, host string) error {
	// invariant: both outputs are /dev/null: the certificate stays in
	// tailscaled's own store, so the first https request after it is answered
	// at once.
	out, err := s.run(ctx, 0, "cert", "--cert-file=/dev/null", "--key-file=/dev/null", host)
	if err != nil {
		return err
	}
	if out.ExitCode != 0 {
		detail := firstLine(out.Stderr + "\n" + out.Stdout)
		if detail == "" {
			detail = "exit " + strconv.Itoa(out.ExitCode)
		}
		return errors.New(detail)
	}
	return nil
}

type rawServe struct {
	TCP map[string]struct {
		HTTPS      bool   `json:"HTTPS"`
		TCPForward string `json:"TCPForward"`
	} `json:"TCP"`
	Web map[string]struct {
		Handlers map[string]struct {
			Proxy string `json:"Proxy"`
		} `json:"Handlers"`
	} `json:"Web"`
}

func (s *Service) ServeStatus(ctx context.Context) (ServeConfig, error) {
	out, err := s.run(ctx, queryBudget, "serve", "status", "--json")
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
	for portText, tcp := range raw.TCP {
		port, err := strconv.Atoi(portText)
		if err == nil && !seen[port] {
			cfg.Entries = append(cfg.Entries, ServeEntry{Port: port, Target: tcp.TCPForward, TCP: tcp.TCPForward != ""})
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

func firstLine(text string) string {
	line, _, _ := strings.Cut(strings.TrimSpace(text), "\n")
	return strings.TrimSpace(line)
}
