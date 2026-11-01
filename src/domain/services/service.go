package services

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/domain/settings"
	"github.com/jterrazz/studio-station/src/system"
)

const fanOutBudget = 5 * time.Second

var ErrDaemonDown = errors.New("the station daemon is not running")

type View struct {
	VM         string     `json:"vm"`
	GuestPort  int        `json:"guest_port"`
	HostPort   int        `json:"host_port"`
	State      Phase      `json:"state"`
	Process    string     `json:"process"`
	HTTP       bool       `json:"http"`
	Pinned     bool       `json:"pinned"`
	Profile    string     `json:"profile"`
	URLs       URLSet     `json:"urls"`
	LastSeenAt *time.Time `json:"last_seen_at"`
	Detail     string     `json:"detail"`
}

type DaemonInfo struct {
	Running bool   `json:"running"`
	Version string `json:"version"`
}

type HostView struct {
	Name       string     `json:"name"`
	Here       bool       `json:"here"`
	Reachable  bool       `json:"reachable"`
	Daemon     DaemonInfo `json:"daemon"`
	PublicHost string     `json:"public_host"`
	Serve      string     `json:"serve"`
	HTTPS      string     `json:"https"`
	Range      *Range     `json:"range,omitempty"`
	Services   []View     `json:"services"`
}

type Range struct {
	Low      int `json:"low"`
	High     int `json:"high"`
	Leased   int `json:"leased"`
	Free     int `json:"free"`
	MaxPerVM int `json:"max_per_vm"`
}

type Fleet struct {
	Schema string     `json:"schema"`
	Hosts  []HostView `json:"hosts"`
}

type Daemon interface {
	Services(ctx context.Context) (HostView, error)
	Reserve(ctx context.Context, vm string, port int, pinned bool) (View, error)
	Release(ctx context.Context, vm string, port int) (Lease, error)
}

type Options struct {
	Here     catalog.MachineKind
	State    string
	HereName string
	VMName   string
	HostName string
	Remote   func(host string, words []string) []string
	Delegate func(ctx context.Context, host string, words []string) (int, error)
	VMHost   func(ctx context.Context, vm string) (host string, here bool, err error)
	Exposure func(ctx context.Context) Exposure
}

type Service struct {
	h     system.Host
	store settings.Store
	d     Daemon
	o     Options
}

func New(h system.Host, store settings.Store, d Daemon, o Options) *Service {
	return &Service{h: h, store: store, d: d, o: o}
}

func (s *Service) inVM() bool { return s.o.Here != catalog.Host }

func (s *Service) policy() Policy {
	cfg, err := s.store.Load()
	if err != nil {
		return PolicyOf(nil)
	}
	return PolicyOf(cfg.Services)
}

func (s *Service) leases() Store { return NewStore(s.h.FS, s.o.State) }

const (
	ServeOn  = "on"
	ServeOff = "off"
)

type Exposure struct {
	On         bool
	PublicHost string
}

func ExposureOf(serveOn bool, dnsName string) Exposure {
	if !serveOn || dnsName == "" {
		return Exposure{PublicHost: LocalHost}
	}
	return Exposure{On: true, PublicHost: dnsName}
}

func (e Exposure) Word() string {
	if e.On {
		return ServeOn
	}
	return ServeOff
}

func ViewOf(l Lease, e Exposure, httpsMode string, phase Phase) View {
	v := View{
		VM: l.VM, GuestPort: l.GuestPort, HostPort: l.HostPort, State: phase, Pinned: l.Pinned, Profile: l.Profile,
		HTTP: l.HTTP != nil && *l.HTTP, URLs: URLs(l, e.PublicHost, httpsMode, e.On),
	}
	if !l.LastSeenAt.IsZero() {
		seen := l.LastSeenAt
		v.LastSeenAt = &seen
	}
	if !e.On {
		v.Detail = catalog.ServeOffDetail
	}
	return v
}

func JoinDetail(a, b string) string {
	switch {
	case a == "":
		return b
	case b == "":
		return a
	}
	return a + " · " + b
}

func (s *Service) offline(ctx context.Context) HostView {
	e := ExposureOf(false, "")
	if s.o.Exposure != nil {
		e = s.o.Exposure(ctx)
	}
	view := HostView{Name: s.o.HereName, Here: true, Reachable: true, PublicHost: e.PublicHost, Serve: e.Word(), HTTPS: s.policy().HTTPS, Services: []View{}}
	leases, _ := s.leases().Load()
	for _, l := range leases {
		view.Services = append(view.Services, ViewOf(l, e, view.HTTPS, Unknown))
	}
	return view
}

func (s *Service) Here(ctx context.Context) (HostView, error) {
	if s.inVM() {
		view, err := s.d.Services(ctx)
		if err != nil {
			return HostView{}, &HostAwayError{Host: s.o.HostName}
		}
		mine := view.Services[:0:0]
		for _, v := range view.Services {
			if v.VM == s.o.VMName {
				mine = append(mine, v)
			}
		}
		view.Services = mine
		return view, nil
	}
	view, err := s.d.Services(ctx)
	if errors.Is(err, ErrDaemonDown) {
		view, err = s.offline(ctx), nil
	}
	if err != nil {
		return HostView{}, err
	}
	view.Name, view.Here, view.Reachable = s.o.HereName, true, true
	view.Range = s.rangeOf()
	return view, nil
}

// invariant: the range is the host's policy and its lease file, so it reads the
// same with the daemon up or down; a lease left outside a narrowed range is not
// counted against it.
func (s *Service) rangeOf() *Range {
	p := s.policy()
	leases, _ := s.leases().Load()
	leased := 0
	for _, l := range leases {
		if l.HostPort >= p.Low && l.HostPort <= p.High {
			leased++
		}
	}
	return &Range{Low: p.Low, High: p.High, Leased: leased, Free: p.High - p.Low + 1 - leased, MaxPerVM: p.MaxPerVM}
}

func (s *Service) CheckList(hereOnly bool) error {
	if hereOnly && s.inVM() {
		return &HereInVMError{}
	}
	return nil
}

func (s *Service) List(ctx context.Context, hereOnly bool) (Fleet, error) {
	here, err := s.Here(ctx)
	if err != nil {
		return Fleet{}, err
	}
	fleet := Fleet{Schema: Schema, Hosts: []HostView{here}}
	if hereOnly || s.inVM() {
		return fleet, nil
	}
	cfg, err := s.store.Load()
	if err != nil {
		return Fleet{}, err
	}
	var others []string
	for name := range cfg.Hosts {
		if name != cfg.Here {
			others = append(others, name)
		}
	}
	sort.Strings(others)
	views := make([]HostView, len(others))
	var wg sync.WaitGroup
	for i, name := range others {
		wg.Go(func() { views[i] = s.ask(ctx, name) })
	}
	wg.Wait()
	fleet.Hosts = append(fleet.Hosts, views...)
	return fleet, nil
}

func (s *Service) ask(ctx context.Context, name string) HostView {
	silent := HostView{Name: name, Services: []View{}}
	argv := s.o.Remote(name, []string{"services", "--here", "--json"})
	out, err := s.h.Run.Output(ctx, system.Cmd{Name: argv[0], Args: argv[1:], Timeout: fanOutBudget})
	if err != nil || out.ExitCode != 0 {
		return silent
	}
	var doc Fleet
	if json.Unmarshal([]byte(out.Stdout), &doc) != nil || len(doc.Hosts) == 0 {
		return silent
	}
	view := doc.Hosts[0]
	view.Name, view.Here, view.Reachable = name, false, true
	if view.Services == nil {
		view.Services = []View{}
	}
	return view
}

func ParsePort(text string) (int, error) {
	n, err := strconv.Atoi(text)
	if err != nil || n < 1 || n > 65535 {
		return 0, &BadPortError{Value: text}
	}
	return n, nil
}

func (s *Service) CheckURL(text string) (int, error) {
	if !s.inVM() {
		return 0, &VMOnlyError{}
	}
	port, err := ParsePort(text)
	if err != nil {
		return 0, err
	}
	if p := s.policy(); p.Ignores(port) {
		return 0, &IgnoredPortError{Port: port, MinPort: p.MinPort}
	}
	return port, nil
}

func (s *Service) URL(ctx context.Context, port int) (View, error) {
	v, err := s.d.Reserve(ctx, s.o.VMName, port, false)
	var down *HostAwayError
	if errors.Is(err, ErrDaemonDown) || errors.As(err, &down) {
		return View{}, &HostAwayError{Host: s.o.HostName}
	}
	return v, err
}

type Target struct {
	VM   string
	Port int
}

func (s *Service) ParseTarget(text string) (Target, error) {
	if s.inVM() {
		port, err := ParsePort(text)
		if err != nil {
			return Target{}, &BadTargetError{Value: text, InVM: true}
		}
		return Target{VM: s.o.VMName, Port: port}, nil
	}
	vm, portText, ok := strings.Cut(text, ":")
	if !ok || vm == "" {
		return Target{}, &BadTargetError{Value: text}
	}
	port, err := ParsePort(portText)
	if err != nil {
		return Target{}, &BadTargetError{Value: text}
	}
	return Target{VM: vm, Port: port}, nil
}

func (s *Service) CheckRelease(ctx context.Context, t Target) (host string, err error) {
	if s.inVM() {
		return "", nil
	}
	host, here, err := s.o.VMHost(ctx, t.VM)
	if err != nil {
		return "", err
	}
	if !here {
		return host, nil
	}
	leases, err := s.leases().Load()
	if err != nil {
		return "", err
	}
	if _, i := Find(leases, t.VM, t.Port); i < 0 {
		return "", &NoLeaseError{VM: t.VM, Port: t.Port}
	}
	return "", nil
}

func (s *Service) Pinned(t Target) bool {
	if s.inVM() {
		return false
	}
	leases, err := s.leases().Load()
	if err != nil {
		return false
	}
	l, i := Find(leases, t.VM, t.Port)
	return i >= 0 && l.Pinned
}

func (s *Service) Release(ctx context.Context, t Target, host string, yes bool) (Lease, error) {
	if host != "" {
		words := []string{"services", "release", t.VM + ":" + strconv.Itoa(t.Port)}
		if yes {
			words = append(words, "--yes")
		}
		code, err := s.o.Delegate(ctx, host, words)
		if err != nil {
			return Lease{}, err
		}
		if code != 0 {
			return Lease{}, &RemoteError{Host: host, Code: code}
		}
		return Lease{VM: t.VM, GuestPort: t.Port}, nil
	}
	l, err := s.d.Release(ctx, t.VM, t.Port)
	if s.inVM() && err != nil && errors.Is(err, ErrDaemonDown) {
		return Lease{}, &HostAwayError{Host: s.o.HostName}
	}
	if !errors.Is(err, ErrDaemonDown) {
		return l, err
	}
	store := s.leases()
	leases, err := store.Load()
	if err != nil {
		return Lease{}, err
	}
	l, i := Find(leases, t.VM, t.Port)
	if i < 0 {
		return Lease{}, &NoLeaseError{VM: t.VM, Port: t.Port}
	}
	return l, store.Save(append(leases[:i:i], leases[i+1:]...))
}
