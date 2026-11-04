package machines

import (
	"context"
	"maps"
	"sort"
	"strings"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/domain/settings"
	"github.com/jterrazz/studio-station/src/machines/contract"
)

func (s *Service) Lookup(_ context.Context, name string) (Machine, error) { return s.lookup(name) }

func (s *Service) lookup(name string) (Machine, error) {
	cfg, err := s.store.Load()
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
	m, err := s.Remote(name)
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
	cfg, err := s.store.Load()
	if err != nil {
		return err
	}
	cfg.Here = name
	if cfg.Hosts == nil {
		cfg.Hosts = map[string]settings.Host{}
	}
	if _, ok := cfg.Hosts[name]; !ok {
		cfg.Hosts[name] = settings.Host{}
	}
	if cfg.VMUser == "" {
		cfg.VMUser = s.o.User
	}
	return s.store.Save(cfg)
}

func (s *Service) AddHost(ctx context.Context, name string, h settings.Host) error {
	if err := s.CheckHost(); err != nil {
		return err
	}
	if !validName(name) {
		return &BadNameError{Name: name}
	}
	cfg, err := s.store.Load()
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
	before := cfg
	cfg.Hosts = maps.Clone(cfg.Hosts)
	if cfg.Hosts == nil {
		cfg.Hosts = map[string]settings.Host{}
	}
	cfg.Hosts[name] = h
	if err := s.store.Save(cfg); err != nil {
		return err
	}
	if err := s.o.SSHConfig.Put(ctx, []contract.HostBlock{{Name: name, User: h.User, Key: h.Key}}); err != nil {
		_ = s.store.Save(before)
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
	cfg, err := s.store.Load()
	if err != nil {
		return err
	}
	delete(cfg.Hosts, m.Name)
	if err := s.store.Save(cfg); err != nil {
		return err
	}
	return configFailure(s.o.SSHConfig.Remove(ctx, m.Name), false)
}

func (s *Service) here(cfg settings.Settings) string {
	if cfg.Here != "" {
		return cfg.Here
	}
	return strings.ToLower(s.h.Env.Hostname())
}

func (s *Service) Names() []string {
	cfg, err := s.store.Load()
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
