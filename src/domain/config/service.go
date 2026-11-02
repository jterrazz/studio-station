package config

import (
	"context"
	"errors"
	"io/fs"
	"path"
	"slices"
	"strings"
	"sync"

	"github.com/jterrazz/studio-station/src/catalog"
	schedule "github.com/jterrazz/studio-station/src/domain/probe"
	"github.com/jterrazz/studio-station/src/domain/settings"
	"github.com/jterrazz/studio-station/src/system"
)

const (
	verbInstall   = "install"
	verbUninstall = "uninstall"
)

type Options struct {
	Pool     *schedule.Pool
	Here     catalog.MachineKind
	Paths    settings.Paths
	Defaults map[string]map[string]string
}

type Service struct {
	h      system.Host
	items  []catalog.ConfigItem
	byName map[string]catalog.ConfigItem
	files  fs.FS
	store  settings.Store
	kc     settings.Keychain
	o      Options
}

func New(h system.Host, items []catalog.ConfigItem, files fs.FS, store settings.Store, o Options) *Service {
	byName := make(map[string]catalog.ConfigItem, len(items))
	for _, it := range items {
		byName[it.Name] = it
	}
	return &Service{h: h, items: items, byName: byName, files: files, store: store, kc: settings.NewKeychain(h.Run).On(o.Here), o: o}
}

type Report struct {
	Item    catalog.ConfigItem
	State   catalog.State
	Detail  string
	Applies bool
}

type Change struct {
	Item   string
	Verb   string
	Result catalog.State
	Before []Fact
	After  []Fact
	Steps  []Step
}

type Fact struct{ Key, Value string }

type Step struct {
	Label  string
	State  catalog.State
	Detail string
}

type handler struct {
	check     func(ctx context.Context) (catalog.State, string)
	install   func(ctx context.Context, values map[string]string) (Change, error)
	uninstall func(ctx context.Context) (Change, error)
}

func (s *Service) Placed() []catalog.ConfigItem {
	var placed []catalog.ConfigItem
	for _, it := range s.items {
		if slices.Contains(it.Places, s.o.Here) {
			placed = append(placed, it)
		}
	}
	return placed
}

func (s *Service) List(ctx context.Context) ([]Report, error) {
	placed := s.Placed()
	reports := make([]Report, len(placed))
	for i, it := range placed {
		h, err := s.handler(it)
		if err != nil {
			return nil, err
		}
		reports[i] = Report{Item: it, State: catalog.StateAbsent, Detail: "run once", Applies: s.applies(it)}
		if h.check == nil {
			continue
		}
		reports[i].State = catalog.StatePending
	}
	// invariant: every item's check is a read of its own, so they run at once and
	// the list costs its slowest check, not their sum.
	var wg sync.WaitGroup
	for i, it := range placed {
		if reports[i].State != catalog.StatePending {
			continue
		}
		h, _ := s.handler(it)
		wg.Go(func() { reports[i].State, reports[i].Detail = h.check(ctx) })
	}
	wg.Wait()
	return reports, nil
}

func (s *Service) Find(names []string) ([]catalog.ConfigItem, error) {
	seen := map[string]bool{}
	var found []catalog.ConfigItem
	for _, name := range names {
		if seen[name] {
			continue
		}
		seen[name] = true
		it, ok := s.byName[name]
		if !ok {
			return nil, &UnknownItemError{Name: name}
		}
		if !slices.Contains(it.Places, s.o.Here) {
			return nil, &WrongPlaceError{Item: name, Places: it.Places, Here: s.o.Here}
		}
		found = append(found, it)
	}
	return found, nil
}

func (s *Service) NeedsInput(it catalog.ConfigItem) bool {
	return len(it.Inputs) > 0 || it.Interactive
}

func (s *Service) Defaults(item string) map[string]string {
	return clone(s.o.Defaults[item])
}

func (s *Service) CheckInstall(names []string, interactive bool) error {
	found, err := s.Find(names)
	if err != nil {
		return err
	}
	for _, it := range found {
		if err := s.admit(it, verbInstall, interactive); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) CheckUninstall(names []string) error {
	found, err := s.Find(names)
	if err != nil {
		return err
	}
	for _, it := range found {
		if err := s.admit(it, verbUninstall, true); err != nil {
			return err
		}
	}
	return nil
}

// invariant: the place, the reversibility, the terminal and the root are local
// facts, judged before any question and before any effect.
func (s *Service) admit(it catalog.ConfigItem, verb string, interactive bool) error {
	switch {
	case !slices.Contains(it.Places, s.o.Here):
		return &WrongPlaceError{Item: it.Name, Places: it.Places, Here: s.o.Here}
	case verb == verbUninstall && !it.Reversible:
		return &NotReversibleError{Item: it.Name}
	case verb == verbInstall && s.NeedsInput(it) && !interactive:
		return &NeedsTerminalError{Item: it.Name}
	case it.NeedsRoot && s.h.Env.Euid() != 0:
		return &NeedsRootError{Item: it.Name, Verb: verb}
	}
	return nil
}

func (s *Service) Install(ctx context.Context, name string, values map[string]string, interactive bool) (Change, error) {
	return s.apply(name, verbInstall, interactive, func(h handler, it catalog.ConfigItem) (Change, error) {
		merged := clone(s.o.Defaults[it.Name])
		for k, v := range values {
			merged[k] = v
		}
		for _, in := range it.Inputs {
			if in.Required && merged[in.Name] == "" {
				return Change{}, errors.New(it.Name + " needs " + in.Name)
			}
		}
		return h.install(ctx, merged)
	})
}

func (s *Service) Uninstall(ctx context.Context, name string) (Change, error) {
	return s.apply(name, verbUninstall, true, func(h handler, _ catalog.ConfigItem) (Change, error) {
		return h.uninstall(ctx)
	})
}

func (s *Service) apply(name, verb string, interactive bool, run func(handler, catalog.ConfigItem) (Change, error)) (Change, error) {
	failed := Change{Item: name, Verb: verb, Result: catalog.StateBroken}
	found, err := s.Find([]string{name})
	if err != nil {
		return failed, err
	}
	it := found[0]
	if refusal := s.admit(it, verb, interactive); refusal != nil {
		return failed, refusal
	}
	h, err := s.handler(it)
	if err != nil {
		return failed, err
	}
	change, err := run(h, it)
	change.Item, change.Verb = it.Name, verb
	if err != nil {
		change.Result = catalog.StateBroken
		change.Steps = append(change.Steps, Step{Label: "failed", State: catalog.StateBroken, Detail: firstLine(err.Error())})
		return change, err
	}
	change.Result = catalog.StateOK
	return change, nil
}

func (s *Service) applies(it catalog.ConfigItem) bool {
	if it.Requires == "" {
		return true
	}
	_, ok := s.h.Run.LookPath(it.Requires)
	return ok
}

func (s *Service) home(p string) string {
	if p == "~" {
		return s.h.Env.Home()
	}
	if rest, ok := strings.CutPrefix(p, "~/"); ok {
		return path.Join(s.h.Env.Home(), rest)
	}
	return p
}

func clone(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func firstLine(s string) string {
	line, _, _ := strings.Cut(strings.TrimSpace(s), "\n")
	return line
}
