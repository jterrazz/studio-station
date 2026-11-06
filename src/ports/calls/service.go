package calls

import (
	"context"
	"errors"
	"github.com/jterrazz/studio-station/src/ports/contract"
	"reflect"
	"sort"

	machinescontract "github.com/jterrazz/studio-station/src/machines/contract"
	setupcontract "github.com/jterrazz/studio-station/src/setup/contract"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/core/store"
	"github.com/jterrazz/studio-station/src/ports"
)

type Daemon interface {
	Services(ctx context.Context) (ports.HostServices, error)
	Reserve(ctx context.Context, vm string, port int, pinned bool) (ports.ServiceView, error)
	Release(ctx context.Context, vm string, port int) (ports.Reservation, error)
}

type ServiceOptions struct {
	Here     catalog.MachineKind
	State    string
	HereName string
	VMName   string
	HostName string
	// value: each other host's own services, from those that answered.
	Ask func(ctx context.Context, hosts []string) map[string]ports.HostServices
	// value: a VM's port released by the host it runs on.
	ReleaseOn func(ctx context.Context, host string, t ports.Target, yes bool) (ports.Reservation, error)
	VMHost    func(ctx context.Context, vm string) (host string, here bool, err error)
	Exposure  func(ctx context.Context) ports.Exposure
	// value: whether this Mac's engine is installed, read when it does not
	// answer.
	EngineInstalled func() bool
	// value: what listens on this Mac, none where nothing can list it; nil
	// reads nothing.
	Listening func(ctx context.Context) ([]ports.Listener, error)
	// value: the routes a declared port is verified by; nil verifies none.
	Routes contract.PublicRoutes
}

type Stores struct {
	Services     contract.PortsPrefs
	Name         setupcontract.NamePrefs
	Hosts        machinescontract.MacsPrefs
	Reservations contract.ReservationStore
	Declarations contract.DeclarationStore
}

type Service struct {
	st Stores
	d  Daemon
	o  ServiceOptions
}

func NewService(st Stores, d Daemon, o ServiceOptions) *Service {
	return &Service{st: st, d: d, o: o}
}

func (s *Service) inVM() bool { return s.o.Here != catalog.Host }

func (s *Service) policy() ports.Policy {
	g, err := s.st.Services.Get()
	if err != nil {
		return ports.PolicyOf(nil)
	}
	return ports.PolicyOf(g)
}

func (s *Service) offline(ctx context.Context) ports.HostServices {
	e := ports.ExposureOf(false, "")
	if s.o.Exposure != nil {
		e = s.o.Exposure(ctx)
	}
	view := ports.HostServices{Name: s.o.HereName, Here: true, Reachable: true, PublicHost: e.PublicHost, Serve: e.Word(), HTTPS: s.policy().HTTPS, Services: []ports.ServiceView{}}
	leases, _ := s.st.Reservations.List(ctx)
	for _, l := range leases {
		view.Services = append(view.Services, ports.ViewOf(l, e, view.HTTPS, ports.PhaseUnknown))
	}
	return view
}

func (s *Service) Here(ctx context.Context) (ports.HostServices, error) {
	if s.inVM() {
		view, err := s.d.Services(ctx)
		if err != nil {
			return ports.HostServices{}, &ports.HostAwayError{Host: s.o.HostName}
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
	if errors.Is(err, ports.ErrDaemonDown) {
		view, err = s.offline(ctx), nil
		view.Daemon.Installed = s.o.EngineInstalled != nil && s.o.EngineInstalled()
	}
	if err != nil {
		return ports.HostServices{}, err
	}
	view.Name, view.Here, view.Reachable = s.o.HereName, true, true
	view.Range = s.rangeOf()
	ps, err := s.survey(ctx, view.Listening)
	if err != nil {
		return ports.HostServices{}, err
	}
	view.Ports, view.Listening = ps, nil
	return view, nil
}

// invariant: this Mac's ports and its VMs', each in its class: the Mac's
// read here, its VMs' as its engine heard them, nothing of a VM while the
// engine is stopped. What listens here that cannot be read fails the read,
// as an empty list would say nothing listens; a Mac with no reader of its
// listeners has nothing to say of them.
func (s *Service) survey(ctx context.Context, vms []ports.Machine) ([]ports.Port, error) {
	var mine []ports.Listener
	if s.o.Listening != nil {
		var err error
		if mine, err = s.o.Listening(ctx); err != nil {
			return nil, err
		}
	}
	here := ports.MacOf(s.o.HereName, mine)
	var decl []ports.Declaration
	if s.st.Declarations != nil {
		decl, _ = s.st.Declarations.List(ctx)
	}
	ms := append([]ports.Machine{here}, vms...)
	ps := ports.Survey(ms, decl)
	if s.o.Routes == nil {
		return ps, nil
	}
	return s.o.Routes.Verify(ctx, ms, ps), nil
}

// invariant: the range is the host's policy and its lease file, so it reads the
// same with the daemon up or down; a lease left outside a narrowed range is not
// counted against it.
func (s *Service) rangeOf() *ports.Range {
	p := s.policy()
	leases, _ := s.st.Reservations.List(context.Background())
	leased := 0
	for _, l := range leases {
		if l.HostPort >= p.Low && l.HostPort <= p.High {
			leased++
		}
	}
	return &ports.Range{Low: p.Low, High: p.High, Leased: leased, Free: p.High - p.Low + 1 - leased, MaxPerVM: p.MaxPerVM}
}

func (s *Service) List(ctx context.Context, hereOnly bool) (ports.Fleet, error) {
	here, err := s.Here(ctx)
	if err != nil {
		return ports.Fleet{}, err
	}
	fleet := ports.Fleet{Schema: ports.LeasesSchema, Hosts: []ports.HostServices{here}}
	if hereOnly || s.inVM() {
		return fleet, nil
	}
	hosts, err := s.st.Hosts.Get()
	if err != nil {
		return ports.Fleet{}, err
	}
	name, err := s.st.Name.Get()
	if err != nil {
		return ports.Fleet{}, err
	}
	var others []string
	for host := range hosts {
		if host != name {
			others = append(others, host)
		}
	}
	sort.Strings(others)
	var answers map[string]ports.HostServices
	if s.o.Ask != nil {
		answers = s.o.Ask(ctx, others)
	}
	views := make([]ports.HostServices, len(others))
	for i, name := range others {
		views[i] = answered(name, answers)
	}
	fleet.Hosts = append(fleet.Hosts, views...)
	return fleet, nil
}

func answered(name string, answers map[string]ports.HostServices) ports.HostServices {
	view, ok := answers[name]
	if !ok {
		return ports.HostServices{Name: name, Services: []ports.ServiceView{}}
	}
	view.Name, view.Here, view.Reachable = name, false, true
	if view.Services == nil {
		view.Services = []ports.ServiceView{}
	}
	return view
}

func (s *Service) Placed() (vm string, inside bool) {
	// value: the VM this binary runs in, as its guest placed it, inside one.
	if !s.inVM() {
		return "", false
	}
	return s.o.VMName, true
}

func (s *Service) CheckPort(text string) (int, error) {
	port, err := ports.ParsePort(text)
	if err != nil {
		return 0, err
	}
	if p := s.policy(); p.Ignores(port) {
		return 0, &ports.IgnoredPortError{Port: port, MinPort: p.MinPort}
	}
	return port, nil
}

func (s *Service) ParsePort(text string) (int, error) { return ports.ParsePort(text) }

func (s *Service) Keeper(ctx context.Context, name string) (named, mac string, err error) {
	// invariant: this Mac keeps its own declarations and its VMs'; another
	// Mac of the fleet keeps its own and its VMs'.
	if name == "" || name == s.o.HereName {
		return s.o.HereName, "", nil
	}
	if hosts, read := s.st.Hosts.Get(); read == nil {
		if _, ok := hosts[name]; ok {
			return name, name, nil
		}
	}
	host, here, err := s.o.VMHost(ctx, name)
	if err != nil || here {
		return name, "", err
	}
	return name, host, nil
}

func (s *Service) CheckVM(ctx context.Context, vm string) (host string, err error) {
	// invariant: a VM of another Mac is shared by that Mac's engine: its
	// name is returned, and its share asked there.
	host, here, err := s.o.VMHost(ctx, vm)
	if err != nil || here {
		return "", err
	}
	return host, nil
}

func (s *Service) Share(ctx context.Context, vm string, port int) (ports.ServiceView, error) {
	v, err := s.d.Reserve(ctx, vm, port, false)
	var down *ports.HostAwayError
	if s.inVM() && (errors.Is(err, ports.ErrDaemonDown) || errors.As(err, &down)) {
		return ports.ServiceView{}, &ports.HostAwayError{Host: s.o.HostName}
	}
	return v, err
}

func (s *Service) CheckRelease(ctx context.Context, t ports.Target) (host string, err error) {
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
	if _, i := ports.Find(leases, t.VM, t.Port); i < 0 {
		return "", ports.NotFoundAt(t)
	}
	return "", nil
}

func (s *Service) Release(ctx context.Context, t ports.Target, host string, yes bool) (ports.Reservation, error) {
	if host != "" {
		return s.o.ReleaseOn(ctx, host, t, yes)
	}
	l, err := s.d.Release(ctx, t.VM, t.Port)
	if s.inVM() && err != nil && errors.Is(err, ports.ErrDaemonDown) {
		return ports.Reservation{}, &ports.HostAwayError{Host: s.o.HostName}
	}
	if !errors.Is(err, ports.ErrDaemonDown) {
		return l, err
	}
	l, err = s.st.Reservations.Get(ctx, t.VM, t.Port)
	if errors.Is(err, store.ErrNotFound) {
		return ports.Reservation{}, ports.NotFoundAt(t)
	}
	if err != nil {
		return ports.Reservation{}, err
	}
	return l, s.st.Reservations.Delete(ctx, t.VM, t.Port)
}

func SaveLeases(ctx context.Context, r contract.ReservationStore, leases []ports.Reservation) error {
	// invariant: saving the whole set changes only what moved: a lease gone is
	// deleted, a lease added or changed is put, each write whole.
	current, err := r.List(ctx)
	if err != nil {
		return err
	}
	want := make(map[string]ports.Reservation, len(leases))
	for _, l := range leases {
		want[l.Key()] = l
	}
	for _, c := range current {
		if _, ok := want[c.Key()]; !ok {
			if err := r.Delete(ctx, c.VM, c.GuestPort); err != nil {
				return err
			}
		}
	}
	had := make(map[string]ports.Reservation, len(current))
	for _, c := range current {
		had[c.Key()] = c
	}
	for _, l := range leases {
		if c, ok := had[l.Key()]; ok && reflect.DeepEqual(c, l) {
			continue
		}
		if err := r.Put(ctx, l); err != nil {
			return err
		}
	}
	return nil
}
