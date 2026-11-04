package services

import (
	"context"
	"errors"
	"github.com/jterrazz/studio-station/src/ports"
	"sort"
	"strconv"
	"strings"

	"github.com/jterrazz/studio-station/src/core/store"
	machinescontract "github.com/jterrazz/studio-station/src/machines/contract"
	portscontract "github.com/jterrazz/studio-station/src/ports/contract"
	setupcontract "github.com/jterrazz/studio-station/src/setup/contract"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/system"
)

var ErrDaemonDown = errors.New("the station daemon is not running")

type (
	View       = ports.ServiceView
	DaemonInfo = ports.EngineInfo
	HostView   = ports.HostServices
	Range      = ports.Range
	Fleet      = ports.Fleet
	Target     = ports.Target
)

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
	// value: each other host's own services, from those that answered.
	Ask func(ctx context.Context, hosts []string) map[string]HostView
	// value: a VM's port released by the host it runs on.
	ReleaseOn func(ctx context.Context, host, target string, yes bool) (Lease, error)
	VMHost    func(ctx context.Context, vm string) (host string, here bool, err error)
	Exposure  func(ctx context.Context) Exposure
}

type Stores struct {
	Services     portscontract.PortsPrefs
	Name         setupcontract.NamePrefs
	Hosts        machinescontract.MacsPrefs
	Reservations Reservations
}

type Service struct {
	h  system.Host
	st Stores
	d  Daemon
	o  Options
}

func New(h system.Host, st Stores, d Daemon, o Options) *Service {
	return &Service{h: h, st: st, d: d, o: o}
}

func (s *Service) inVM() bool { return s.o.Here != catalog.Host }

func (s *Service) policy() Policy {
	g, err := s.st.Services.Get()
	if err != nil {
		return PolicyOf(nil)
	}
	return PolicyOf(g)
}

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
	return a + ", " + b
}

func (s *Service) offline(ctx context.Context) HostView {
	e := ExposureOf(false, "")
	if s.o.Exposure != nil {
		e = s.o.Exposure(ctx)
	}
	view := HostView{Name: s.o.HereName, Here: true, Reachable: true, PublicHost: e.PublicHost, Serve: e.Word(), HTTPS: s.policy().HTTPS, Services: []View{}}
	leases, _ := s.st.Reservations.List(ctx)
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
		_, missing := s.h.FS.Stat(catalog.DaemonPlist(s.h.Env.Home()))
		view.Daemon.Installed = missing == nil
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
	leases, _ := s.st.Reservations.List(context.Background())
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
	hosts, err := s.st.Hosts.Get()
	if err != nil {
		return Fleet{}, err
	}
	name, err := s.st.Name.Get()
	if err != nil {
		return Fleet{}, err
	}
	var others []string
	for host := range hosts {
		if host != name {
			others = append(others, host)
		}
	}
	sort.Strings(others)
	var answers map[string]HostView
	if s.o.Ask != nil {
		answers = s.o.Ask(ctx, others)
	}
	views := make([]HostView, len(others))
	for i, name := range others {
		views[i] = answered(name, answers)
	}
	fleet.Hosts = append(fleet.Hosts, views...)
	return fleet, nil
}

func answered(name string, answers map[string]HostView) HostView {
	view, ok := answers[name]
	if !ok {
		return HostView{Name: name, Services: []View{}}
	}
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
	leases, err := s.st.Reservations.List(ctx)
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
	leases, err := s.st.Reservations.List(context.Background())
	if err != nil {
		return false
	}
	l, i := Find(leases, t.VM, t.Port)
	return i >= 0 && l.Pinned
}

func (s *Service) Release(ctx context.Context, t Target, host string, yes bool) (Lease, error) {
	if host != "" {
		return s.o.ReleaseOn(ctx, host, t.VM+":"+strconv.Itoa(t.Port), yes)
	}
	l, err := s.d.Release(ctx, t.VM, t.Port)
	if s.inVM() && err != nil && errors.Is(err, ErrDaemonDown) {
		return Lease{}, &HostAwayError{Host: s.o.HostName}
	}
	if !errors.Is(err, ErrDaemonDown) {
		return l, err
	}
	l, err = s.st.Reservations.Get(ctx, t.VM, t.Port)
	if errors.Is(err, store.ErrNotFound) {
		return Lease{}, &NoLeaseError{VM: t.VM, Port: t.Port}
	}
	if err != nil {
		return Lease{}, err
	}
	return l, s.st.Reservations.Delete(ctx, t.VM, t.Port)
}
