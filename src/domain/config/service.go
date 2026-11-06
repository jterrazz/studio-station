package config

import (
	"context"
	"errors"
	"github.com/jterrazz/studio-station/src/settings"
	"slices"

	"github.com/jterrazz/studio-station/src/catalog"
	schedule "github.com/jterrazz/studio-station/src/core/call/lanes"
	settingscontract "github.com/jterrazz/studio-station/src/settings/contract"
	"github.com/jterrazz/studio-station/src/system"
)

const (
	verbInstall   = "install"
	verbUninstall = "uninstall"
)

type Options struct {
	Here     catalog.MachineKind
	Defaults map[string]map[string]string
	// value: the Applier of every effect of the catalogue.
	Appliers []settingscontract.Applier
}

type Service struct {
	h        system.Host
	items    []catalog.Setting
	byName   map[string]catalog.Setting
	o        Options
	appliers settingscontract.Appliers
}

func New(h system.Host, items []catalog.Setting, o Options) *Service {
	byName := make(map[string]catalog.Setting, len(items))
	for _, it := range items {
		byName[it.Name] = it
	}
	return &Service{h: h, items: items, byName: byName, o: o, appliers: settingscontract.NewAppliers(o.Appliers...)}
}

type (
	Report = settings.Report
	Change = settings.Change
	Fact   = settings.KeyValue
	Step   = settings.ChangeStep
)

type handler struct {
	check     func(ctx context.Context) (catalog.State, string)
	install   func(ctx context.Context, values map[string]string) (Change, error)
	uninstall func(ctx context.Context) (Change, error)
}

func (s *Service) Placed() []catalog.Setting {
	var placed []catalog.Setting
	for _, it := range s.items {
		if slices.Contains(it.Kinds, s.o.Here) {
			placed = append(placed, it)
		}
	}
	return placed
}

func (s *Service) Items() []catalog.Setting { return s.items }

func (s *Service) Lives(it catalog.Setting) bool { return slices.Contains(it.Kinds, s.o.Here) }

func (s *Service) Needs(it catalog.Setting) string {
	if s.applies(it) {
		return ""
	}
	return it.Requires
}

func (s *Service) Admit(name string, interactive bool) error {
	found, err := s.Find([]string{name})
	if err != nil {
		return err
	}
	return s.admit(found[0], verbInstall, interactive)
}

func (s *Service) Apply(ctx context.Context, name string, values map[string]string, interactive bool) (Change, error) {
	return s.Install(ctx, name, values, interactive)
}

func (s *Service) List(ctx context.Context) ([]Report, error) {
	reports, err := s.Watch(ctx)
	if err != nil {
		return nil, err
	}
	return schedule.Last(reports), nil
}

func (s *Service) Watch(ctx context.Context) (<-chan []Report, error) {
	// invariant: the first value is every placed item, those with a check
	// pending; every check is a read of its own, so they run at once, each row
	// fills as its check answers, and the list costs its slowest check.
	placed := s.Placed()
	reports := make([]Report, len(placed))
	checks := make([]func(context.Context) (catalog.State, string), len(placed))
	for i, it := range placed {
		h, err := s.handler(it)
		if err != nil {
			return nil, err
		}
		reports[i] = Report{Item: it, State: catalog.StateAbsent, Detail: "run once", Applies: s.applies(it)}
		if h.check != nil {
			reports[i].State, reports[i].Detail, checks[i] = catalog.StatePending, "", h.check
		}
	}
	type answer struct {
		i      int
		state  catalog.State
		detail string
	}
	answers := make(chan answer, len(placed))
	left := 0
	for i, check := range checks {
		if check == nil {
			continue
		}
		left++
		go func() {
			state, detail := check(ctx)
			answers <- answer{i: i, state: state, detail: detail}
		}()
	}
	out := make(chan []Report)
	go func() {
		defer close(out)
		send := func() bool {
			select {
			case out <- slices.Clone(reports):
				return true
			case <-ctx.Done():
				return false
			}
		}
		if !send() {
			return
		}
		for ; left > 0; left-- {
			a := <-answers
			reports[a.i].State, reports[a.i].Detail = a.state, a.detail
			if !send() {
				return
			}
		}
	}()
	return out, nil
}

func (s *Service) Find(names []string) ([]catalog.Setting, error) {
	seen := map[string]bool{}
	var found []catalog.Setting
	for _, name := range names {
		if seen[name] {
			continue
		}
		seen[name] = true
		it, ok := s.byName[name]
		if !ok {
			return nil, &UnknownItemError{Name: name}
		}
		if !slices.Contains(it.Kinds, s.o.Here) {
			return nil, &WrongPlaceError{Item: name, Places: it.Kinds, Here: s.o.Here}
		}
		found = append(found, it)
	}
	return found, nil
}

func (s *Service) NeedsInput(it catalog.Setting) bool {
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
func (s *Service) admit(it catalog.Setting, verb string, interactive bool) error {
	switch {
	case !slices.Contains(it.Kinds, s.o.Here):
		return &WrongPlaceError{Item: it.Name, Places: it.Kinds, Here: s.o.Here}
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
	return s.apply(name, verbInstall, interactive, func(h handler, it catalog.Setting) (Change, error) {
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
	return s.apply(name, verbUninstall, true, func(h handler, _ catalog.Setting) (Change, error) {
		return h.uninstall(ctx)
	})
}

func (s *Service) apply(name, verb string, interactive bool, run func(handler, catalog.Setting) (Change, error)) (Change, error) {
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

func (s *Service) applies(it catalog.Setting) bool {
	if it.Requires == "" {
		return true
	}
	_, ok := s.h.Run.LookPath(it.Requires)
	return ok
}

func clone(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
