package tools

import (
	"context"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/system"
)

type Options struct {
	Here        catalog.MachineKind
	MiseShims   string
	Path        string
	APIURL      string
	GitHubToken string
}

type Configurer interface {
	Configure(ctx context.Context, item string, interactive bool) (Step, error)
}

type Service struct {
	h      system.Host
	tools  []catalog.Tool
	byName map[string]catalog.Tool
	cfg    Configurer
	o      Options
	table  map[catalog.Method]method
}

func New(h system.Host, list []catalog.Tool, cfg Configurer, o Options) *Service {
	byName := make(map[string]catalog.Tool, len(list))
	for _, t := range list {
		byName[t.Name] = t
	}
	return &Service{h: h, tools: list, byName: byName, cfg: cfg, o: o, table: methods()}
}

type Report struct {
	Tool    catalog.Tool
	Method  catalog.Method
	State   catalog.State
	Version string
	Managed bool
	Detail  string
	Live    string
}

type Outcome struct {
	Name   string
	Verb   string
	Result catalog.State
	Detail string
	Steps  []Step
	Err    error
}

type Step struct {
	Item   string
	Result catalog.State
	Detail string
}

type InstallRequest struct {
	Names       []string
	Interactive bool
}

type UpgradeRequest struct {
	Names []string
	All   bool
	Via   []catalog.Manager
}

func (s *Service) Placed() []catalog.Tool {
	var placed []catalog.Tool
	for _, t := range s.tools {
		if _, ok := t.InstallOn(s.o.Here); ok {
			placed = append(placed, t)
		}
	}
	return placed
}

func (s *Service) Find(names []string) ([]catalog.Tool, error) {
	seen := map[string]bool{}
	var found []catalog.Tool
	for _, name := range names {
		if seen[name] {
			continue
		}
		seen[name] = true
		t, ok := s.byName[name]
		if !ok {
			return nil, &UnknownToolError{Name: name}
		}
		if _, ok := t.InstallOn(s.o.Here); !ok {
			return nil, &WrongPlaceError{Tool: name, Places: t.Places(), Here: s.o.Here}
		}
		found = append(found, t)
	}
	return found, nil
}

func (s *Service) CheckInstall(names []string) error {
	found, err := s.Find(names)
	if err != nil {
		return err
	}
	for _, t := range found {
		i := s.install(t)
		if app, ok := i.(catalog.AppInstall); ok {
			return &ManualInstallError{Tool: t.Name, Hint: app.Hint}
		}
	}
	return nil
}

func (s *Service) CheckUninstall(names []string) error {
	found, err := s.Find(names)
	if err != nil {
		return err
	}
	for _, t := range found {
		if s.row(t).uninstall == nil {
			return &NotReversibleError{Tool: t.Name}
		}
	}
	return nil
}

func (s *Service) CheckUpgrade(names []string) error {
	found, err := s.Find(names)
	if err != nil {
		return err
	}
	for _, t := range found {
		if !s.row(t).upgradable() {
			return &NoUpgradePathError{Tool: t.Name}
		}
	}
	return nil
}

func (s *Service) install(t catalog.Tool) catalog.Install {
	i, _ := t.InstallOn(s.o.Here)
	return i
}

func (s *Service) row(t catalog.Tool) method {
	return s.table[s.install(t).Method()]
}
