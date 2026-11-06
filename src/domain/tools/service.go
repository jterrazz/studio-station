package tools

import (
	"context"
	"errors"
	"slices"
	"sort"

	"github.com/jterrazz/studio-station/src/apps"
	"github.com/jterrazz/studio-station/src/apps/contract"
	"github.com/jterrazz/studio-station/src/catalog"
	probe "github.com/jterrazz/studio-station/src/core/call/lanes"
	"github.com/jterrazz/studio-station/src/system"
)

const commandLane = 16

type Options struct {
	Pool       *probe.Pool
	Here       catalog.MachineKind
	Root       string
	Installers contract.Installers
	Bundler    contract.Bundler
	VMs        VMs
	Log        string
}

type Configurer interface {
	Configure(ctx context.Context, item string, interactive bool) (Step, error)
}

type VMs interface {
	Running(ctx context.Context) (map[string]catalog.MachineKind, error)
}

type Service struct {
	h      system.Host
	tools  []catalog.App
	byName map[string]catalog.App
	cfg    Configurer
	o      Options
}

func New(h system.Host, list []catalog.App, cfg Configurer, o Options) *Service {
	byName := make(map[string]catalog.App, len(list))
	for _, t := range list {
		byName[t.Name] = t
	}
	if o.Pool == nil {
		o.Pool = probe.NewPool(context.Background(), h.Clock, commandLane, 1)
	}
	return &Service{h: h, tools: list, byName: byName, cfg: cfg, o: o}
}

type (
	Report         = apps.Report
	Why            = apps.Why
	Outcome        = apps.Outcome
	Step           = apps.Step
	InstallRequest = apps.InstallRequest
	UpgradeRequest = apps.UpgradeRequest
)

const (
	WhyMissing   = apps.WhyMissing
	WhyUnmanaged = apps.WhyUnmanaged
	WhyOutdated  = apps.WhyOutdated
	WhyPartial   = apps.WhyPartial
	WhyLate      = apps.WhyLate
	WhyFailing   = apps.WhyFailing
)

func (s *Service) Placed() []catalog.App {
	var placed []catalog.App
	for _, t := range s.tools {
		if _, ok := t.InstallOn(s.o.Here); ok {
			placed = append(placed, t)
		}
	}
	return placed
}

func (s *Service) Find(names []string) ([]catalog.App, error) {
	seen := map[string]bool{}
	var found []catalog.App
	for _, name := range names {
		if seen[name] {
			continue
		}
		seen[name] = true
		t, ok := s.byName[name]
		if !ok {
			return nil, &apps.UnknownError{Name: name}
		}
		if _, ok := t.InstallOn(s.o.Here); !ok {
			return nil, &apps.ElsewhereError{App: name, Kinds: t.LivesOn()}
		}
		found = append(found, t)
	}
	return found, nil
}

func (s *Service) CheckInstall(ctx context.Context, names []string) error {
	found, err := s.Find(names)
	var place *apps.ElsewhereError
	if errors.As(err, &place) {
		place.VM = s.runningVM(ctx, place.Kinds)
	}
	if err != nil {
		return err
	}
	asked := map[string]bool{}
	for _, t := range found {
		asked[t.Name] = true
	}
	for _, t := range found {
		inst := s.installer(t)
		if !inst.Can().Install {
			return &ManualInstallError{Tool: t.Name, Hint: s.install(t).Key()}
		}
		// invariant: an app whose method's manager is missing is refused,
		// naming the app that brings it, unless that app is asked too.
		if app := inst.Manager().App; !asked[app] && errors.Is(inst.Ready(ctx), contract.ErrMissing) {
			return &apps.NeedsError{App: t.Name, Needs: app}
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
		if !s.installer(t).Can().Remove {
			return &NotReversibleError{Tool: t.Name}
		}
		if s.fromAppStore(t) {
			return &AppStoreError{Tool: t.Name}
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
		if !s.installer(t).Can().Update {
			return &NoUpgradePathError{Tool: t.Name}
		}
	}
	return nil
}

func (s *Service) install(t catalog.App) catalog.Install {
	i, _ := t.InstallOn(s.o.Here)
	return i
}

// invariant: the registry wire builds holds an installer for every method
// the catalogue places.
func (s *Service) installer(t catalog.App) contract.Installer {
	return s.o.Installers[s.install(t).Method()]
}

func (s *Service) lined(t catalog.App) (contract.Lined, bool) {
	l, ok := s.installer(t).(contract.Lined)
	return l, ok
}

// invariant: the VM named in a refusal is one that runs now and whose kind is a
// place of the tool; a failed listing names none, so the refusal still stands.
func (s *Service) runningVM(ctx context.Context, places []catalog.MachineKind) string {
	if s.o.VMs == nil {
		return ""
	}
	running, err := s.o.VMs.Running(ctx)
	if err != nil {
		return ""
	}
	names := make([]string, 0, len(running))
	for name, kind := range running {
		if slices.Contains(places, kind) {
			names = append(names, name)
		}
	}
	if len(names) == 0 {
		return ""
	}
	sort.Strings(names)
	return names[0]
}
