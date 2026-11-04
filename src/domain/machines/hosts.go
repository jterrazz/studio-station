package machines

import (
	"context"
	"errors"
	"maps"
	"sort"
	"strings"
	"time"

	model "github.com/jterrazz/studio-station/src/machines"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/machines/contract"
)

type Added = model.Added

func (s *Service) Lookup(_ context.Context, name string) (Machine, error) { return s.lookup(name) }

func (s *Service) lookup(name string) (Machine, error) {
	cfg, err := s.prefs()
	if err != nil {
		return Machine{}, err
	}
	if h, ok := cfg.Hosts[name]; ok {
		return Machine{Name: name, Kind: catalog.Host, Here: name == cfg.Here, User: h.User, Key: h.Key}, nil
	}
	records, err := s.records()
	if err != nil {
		return Machine{}, err
	}
	for _, r := range records {
		if r.Name == name {
			return Machine{Name: name, Kind: kindOfImage(r.Image), Host: cfg.Here, User: s.vmUser(cfg), Lifetime: r.Lifetime}, nil
		}
	}
	// value: a Mac answers to its short name too, the one All machines shows
	// (mac-mini for jterrazz-mac-mini).
	if full := s.o.User + "-" + name; s.o.User != "" {
		if h, ok := cfg.Hosts[full]; ok {
			return Machine{Name: full, Kind: catalog.Host, Here: full == cfg.Here, User: h.User, Key: h.Key}, nil
		}
	}
	return Machine{}, &UnknownMachineError{Name: name}
}

func (s *Service) Remote(name string) (Machine, error) {
	m, err := s.lookup(name)
	if err != nil {
		return Machine{}, err
	}
	if m.Here {
		return Machine{}, &HereMachineError{Name: name}
	}
	return m, nil
}

func (s *Service) RemoteHost(name, verb string) (Machine, error) {
	// invariant: a verb of another Mac refuses This Mac by any of its names,
	// registered or not, before it looks the name up.
	if cfg, err := s.prefs(); err == nil {
		if here := s.here(cfg); name == here || (s.o.User != "" && s.o.User+"-"+name == here) {
			return Machine{}, &HereMachineError{Name: name, Verb: verb}
		}
	}
	m, err := s.Remote(name)
	var here *HereMachineError
	if errors.As(err, &here) {
		here.Verb = verb
	}
	if err != nil {
		return Machine{}, err
	}
	if m.Kind != catalog.Host {
		return Machine{}, &NotAHostError{Name: name, Verb: verb}
	}
	return m, nil
}

func (s *Service) Init(name string) error {
	if !validName(name) {
		return &BadNameError{Name: name}
	}
	cfg, err := s.prefs()
	if err != nil {
		return err
	}
	if err := s.p.Name.Set(name); err != nil {
		return err
	}
	if _, ok := cfg.Hosts[name]; !ok {
		hosts := maps.Clone(cfg.Hosts)
		if hosts == nil {
			hosts = map[string]model.Host{}
		}
		hosts[name] = model.Host{}
		if err := s.p.Hosts.Set(hosts); err != nil {
			return err
		}
	}
	if cfg.VMUser == "" {
		return s.p.VMUser.Set(s.o.User)
	}
	return nil
}

func (s *Service) AddHost(ctx context.Context, name string, h model.Host) error {
	if err := s.CheckHost(); err != nil {
		return err
	}
	if !validName(name) {
		return &BadNameError{Name: name}
	}
	cfg, err := s.prefs()
	if err != nil {
		return err
	}
	if _, ok := cfg.Hosts[name]; ok {
		return &NameTakenError{Name: name}
	}
	records, err := s.records()
	if err != nil {
		return err
	}
	for _, r := range records {
		if r.Name == name {
			return &NameTakenError{Name: name, VM: true}
		}
	}
	before := cfg.Hosts
	hosts := maps.Clone(cfg.Hosts)
	if hosts == nil {
		hosts = map[string]model.Host{}
	}
	hosts[name] = h
	if err := s.p.Hosts.Set(hosts); err != nil {
		return err
	}
	if err := s.o.SSHConfig.Put(ctx, []contract.HostBlock{{Name: name, User: h.User, Key: h.Key}}); err != nil {
		_ = s.p.Hosts.Set(before)
		return configFailure(err, true)
	}
	return nil
}

func (s *Service) ForgetHost(ctx context.Context, name string) error {
	if err := s.CheckHost(); err != nil {
		return err
	}
	m, err := s.RemoteHost(name, "forget")
	if err != nil {
		return err
	}
	cfg, err := s.prefs()
	if err != nil {
		return err
	}
	delete(cfg.Hosts, m.Name)
	if err := s.p.Hosts.Set(cfg.Hosts); err != nil {
		return err
	}
	return configFailure(s.o.SSHConfig.Remove(ctx, m.Name), false)
}

func (s *Service) here(cfg prefsView) string {
	if cfg.Here != "" {
		return cfg.Here
	}
	return strings.ToLower(s.h.Env.Hostname())
}

func (s *Service) OtherHosts() []string {
	// value: every host of the fleet but this one, by name.
	cfg, err := s.prefs()
	if err != nil {
		return nil
	}
	here := s.here(cfg)
	others := make([]string, 0, len(cfg.Hosts))
	for n := range cfg.Hosts {
		if n != here && n != cfg.Here {
			others = append(others, n)
		}
	}
	sort.Strings(others)
	return others
}

func (s *Service) Names() []string {
	cfg, err := s.prefs()
	if err != nil {
		return nil
	}
	names := make([]string, 0, len(cfg.Hosts))
	for n := range cfg.Hosts {
		names = append(names, n)
	}
	if records, err := s.records(); err == nil {
		for _, r := range records {
			names = append(names, r.Name)
		}
	}
	sort.Strings(names)
	return names
}

func (s *Service) Remember(_ context.Context, f Fleet) error {
	cfg, err := s.prefs()
	if err != nil {
		return err
	}
	changed := false
	for _, h := range f.Hosts {
		known, ok := cfg.Hosts[h.Name]
		if h.Here || !h.Reachable || !ok {
			continue
		}
		seen := make([]model.SeenVM, 0, len(h.VMs))
		for _, vm := range model.Ordered(h.VMs) {
			seen = append(seen, model.SeenVM{Name: vm.Name, Lifetime: vm.Lifetime, CPUs: vm.Resources.CPUs, MemoryBytes: vm.Resources.MemoryBytes})
		}
		known.Answered, known.VMs = s.h.Clock.Now().UTC().Truncate(time.Second), seen
		cfg.Hosts[h.Name], changed = known, true
	}
	if !changed {
		return nil
	}
	return s.p.Hosts.Set(cfg.Hosts)
}
