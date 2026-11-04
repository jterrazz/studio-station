package tools

import (
	"context"
	"slices"

	"github.com/jterrazz/studio-station/src/apps/contract"
	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/core/call"
)

const (
	verbInstall   = "install"
	verbUninstall = "uninstall"
	verbUpgrade   = "upgrade"
)

func (s *Service) Install(ctx context.Context, r InstallRequest) ([]Outcome, error) {
	requested, err := s.Find(r.Names)
	if err != nil {
		return nil, err
	}
	states := s.statesWithRequirements(ctx, requested)
	inRequest := map[string]bool{}
	for _, t := range requested {
		inRequest[t.Name] = true
	}
	for _, t := range requested {
		for _, dep := range s.requirements(t) {
			if !inRequest[dep] && !installed(states[dep]) {
				return nil, &MissingDependencyError{Tool: t.Name, Requires: dep}
			}
		}
	}
	ordered := s.topological(requested)
	outcomes := map[string]*Outcome{}
	var pending []catalog.Tool
	for _, t := range ordered {
		if states[t.Name] == catalog.StateOK {
			outcomes[t.Name] = &Outcome{Name: t.Name, Verb: verbInstall, Result: catalog.StateOK, Detail: "already installed"}
			continue
		}
		pending = append(pending, t)
	}
	for len(pending) > 0 {
		var batch []catalog.Tool
		if _, ok := s.lined(pending[0]); ok {
			batch = s.bundleBatch(pending, inRequest, outcomes)
		} else {
			batch = []catalog.Tool{pending[0]}
		}
		pending = slices.DeleteFunc(pending, func(t catalog.Tool) bool { return slices.ContainsFunc(batch, sameName(t)) })
		var ready []catalog.Tool
		for _, t := range batch {
			if dep := s.failedRequirement(t, outcomes); dep != "" {
				outcomes[t.Name] = &Outcome{Name: t.Name, Verb: verbInstall, Result: catalog.StateBroken, Detail: "requires " + dep + ", which failed"}
				continue
			}
			ready = append(ready, t)
		}
		if len(ready) == 0 {
			continue
		}
		var done []Outcome
		if _, ok := s.lined(ready[0]); ok {
			done = s.bundle(ctx, verbInstall, ready, false)
		} else {
			done = []Outcome{s.run(ctx, verbInstall, ready[0], contract.Installer.Install)}
		}
		for i := range done {
			o := done[i]
			if o.Result == catalog.StateOK {
				o.Steps = s.then(ctx, s.byName[o.Name], r.Interactive)
			}
			outcomes[o.Name] = &o
		}
	}
	result := make([]Outcome, 0, len(ordered))
	for _, t := range requested {
		result = append(result, *outcomes[t.Name])
	}
	return result, nil
}

func (s *Service) Uninstall(ctx context.Context, names []string) ([]Outcome, error) {
	requested, err := s.Find(names)
	if err != nil {
		return nil, err
	}
	states := s.states(ctx, requested)
	out := make([]Outcome, 0, len(requested))
	for _, t := range requested {
		if !installed(states[t.Name]) {
			out = append(out, Outcome{Name: t.Name, Verb: verbUninstall, Result: catalog.StateAbsent, Detail: "not installed"})
			continue
		}
		out = append(out, s.run(ctx, verbUninstall, t, contract.Installer.Remove))
	}
	return out, nil
}

func (s *Service) Upgrade(ctx context.Context, r UpgradeRequest) ([]Outcome, error) {
	var candidates []catalog.Tool
	if len(r.Names) > 0 {
		found, err := s.Find(r.Names)
		if err != nil {
			return nil, err
		}
		candidates = found
	} else {
		via := map[catalog.Method]bool{}
		for _, mm := range catalog.Managers {
			if slices.Contains(r.Via, mm.Manager) {
				for _, m := range mm.Methods {
					via[m] = true
				}
			}
		}
		for _, t := range s.Placed() {
			can := s.installer(t).Can()
			reached := can.UpdateAll
			if len(r.Via) > 0 {
				reached = via[s.install(t).Method()]
			}
			if reached && can.Update {
				candidates = append(candidates, t)
			}
		}
	}
	reports := map[string]Report{}
	for _, rep := range s.reports(ctx, candidates) {
		reports[rep.Tool.Name] = rep
	}
	var outcomes []Outcome
	var bundled, single []catalog.Tool
	for _, t := range candidates {
		state := reports[t.Name].State
		upgradable := state == catalog.StateOK || (len(r.Names) > 0 && state == catalog.StateDegraded)
		switch {
		case reports[t.Name].Detail == DetailAppStore && len(r.Names) > 0:
			outcomes = append(outcomes, Outcome{Name: t.Name, Verb: verbUpgrade, Result: catalog.StateAbsent, Detail: "the App Store updates it"})
		case reports[t.Name].Detail == DetailAppStore:
		case !upgradable && len(r.Names) > 0:
			outcomes = append(outcomes, Outcome{Name: t.Name, Verb: verbUpgrade, Result: catalog.StateAbsent, Detail: "not installed: station tools install " + t.Name})
		case !upgradable:
		case s.isLined(t):
			bundled = append(bundled, t)
		default:
			single = append(single, t)
		}
	}
	if len(bundled) > 0 {
		outcomes = append(outcomes, s.bundle(ctx, verbUpgrade, bundled, true)...)
	}
	for _, t := range single {
		outcomes = append(outcomes, s.run(ctx, verbUpgrade, t, contract.Installer.Update))
	}
	return outcomes, nil
}

func (s *Service) bundleBatch(pending []catalog.Tool, inRequest map[string]bool, done map[string]*Outcome) []catalog.Tool {
	var batch []catalog.Tool
	for _, t := range pending {
		if !s.isLined(t) {
			continue
		}
		ready := true
		for _, dep := range s.requirements(t) {
			if inRequest[dep] && done[dep] == nil && !slices.ContainsFunc(batch, func(b catalog.Tool) bool { return b.Name == dep }) {
				ready = false
			}
		}
		if ready {
			batch = append(batch, t)
		}
	}
	return batch
}

func (s *Service) isLined(t catalog.Tool) bool {
	_, ok := s.lined(t)
	return ok
}

func (s *Service) bundle(ctx context.Context, verb string, list []catalog.Tool, update bool) []Outcome {
	lines := make([]string, 0, len(list))
	for _, t := range list {
		l, _ := s.lined(t)
		lines = append(lines, l.Line(s.install(t)))
	}
	failure := ""
	if err := s.o.Bundler.Bundle(ctx, lines, update, nil); err != nil {
		failure = err.Error()
	}
	out := make([]Outcome, 0, len(list))
	if verb == verbUpgrade {
		for _, t := range list {
			o := Outcome{Name: t.Name, Verb: verb, Result: catalog.StateOK}
			if failure != "" {
				o.Result, o.Detail = catalog.StateBroken, failure
			}
			out = append(out, o)
		}
		return out
	}
	// invariant: one exit code answers for N tools, so each tool's verdict is
	// whether the inventory lists it once the bundle has run.
	states := s.states(ctx, list)
	for _, t := range list {
		o := Outcome{Name: t.Name, Verb: verb, Result: catalog.StateOK}
		if states[t.Name] != catalog.StateOK {
			o.Result, o.Detail = catalog.StateBroken, failure
			if failure == "" {
				o.Detail = "not listed after " + s.o.Bundler.Name()
			}
		}
		out = append(out, o)
	}
	return out
}

type verbOf func(contract.Installer, context.Context, []catalog.Install, call.Progress) error

func (s *Service) run(ctx context.Context, verb string, t catalog.Tool, op verbOf) Outcome {
	o := Outcome{Name: t.Name, Verb: verb, Result: catalog.StateOK}
	if err := op(s.installer(t), ctx, []catalog.Install{s.install(t)}, nil); err != nil {
		o.Result, o.Detail = catalog.StateBroken, err.Error()
	}
	return o
}

func (s *Service) then(ctx context.Context, t catalog.Tool, interactive bool) []Step {
	steps := make([]Step, 0, len(t.Then))
	for _, item := range t.Then {
		if s.cfg == nil {
			steps = append(steps, Step{Item: item, Result: catalog.StateAbsent, Detail: "not applied: station config install " + item})
			continue
		}
		step, err := s.cfg.Configure(ctx, item, interactive)
		if err != nil {
			step = Step{Item: item, Result: catalog.StateBroken, Detail: err.Error()}
		}
		steps = append(steps, step)
	}
	return steps
}

func (s *Service) requirements(t catalog.Tool) []string {
	reqs := slices.Clone(t.Requires)
	if needs := s.installer(t).Manager().Needs; needs != "" && needs != t.Name && !slices.Contains(reqs, needs) {
		reqs = append([]string{needs}, reqs...)
	}
	return reqs
}

func (s *Service) failedRequirement(t catalog.Tool, outcomes map[string]*Outcome) string {
	for _, dep := range s.requirements(t) {
		if o := outcomes[dep]; o != nil && o.Result == catalog.StateBroken {
			return dep
		}
	}
	return ""
}

func (s *Service) topological(list []catalog.Tool) []catalog.Tool {
	inRequest := map[string]catalog.Tool{}
	for _, t := range list {
		inRequest[t.Name] = t
	}
	var ordered []catalog.Tool
	visited := map[string]bool{}
	var visit func(t catalog.Tool)
	visit = func(t catalog.Tool) {
		if visited[t.Name] {
			return
		}
		visited[t.Name] = true
		for _, dep := range s.requirements(t) {
			if d, ok := inRequest[dep]; ok {
				visit(d)
			}
		}
		ordered = append(ordered, t)
	}
	for _, t := range list {
		visit(t)
	}
	return ordered
}

func (s *Service) States(ctx context.Context, names []string) (map[string]catalog.State, error) {
	list, err := s.Find(names)
	if err != nil {
		return nil, err
	}
	return s.states(ctx, list), nil
}

func (s *Service) states(ctx context.Context, list []catalog.Tool) map[string]catalog.State {
	out := map[string]catalog.State{}
	for _, r := range s.reports(ctx, list) {
		out[r.Tool.Name] = r.State
	}
	return out
}

func (s *Service) statesWithRequirements(ctx context.Context, list []catalog.Tool) map[string]catalog.State {
	all := map[string]catalog.Tool{}
	var walk func(t catalog.Tool)
	walk = func(t catalog.Tool) {
		if _, ok := all[t.Name]; ok {
			return
		}
		all[t.Name] = t
		for _, dep := range s.requirements(t) {
			if d, ok := s.byName[dep]; ok {
				if _, placed := d.InstallOn(s.o.Here); placed {
					walk(d)
				}
			}
		}
	}
	for _, t := range list {
		walk(t)
	}
	flat := make([]catalog.Tool, 0, len(all))
	for _, t := range all {
		flat = append(flat, t)
	}
	return s.states(ctx, flat)
}

func installed(s catalog.State) bool { return s == catalog.StateOK || s == catalog.StateDegraded }

func sameName(t catalog.Tool) func(catalog.Tool) bool {
	return func(o catalog.Tool) bool { return o.Name == t.Name }
}

func (s *Service) Superseded(ctx context.Context) []string {
	// invariant: a superseded formula goes only once the tool replacing it is
	// ok: the formula alone is a half install, named, never removed.
	var list []catalog.Tool
	for _, t := range s.Placed() {
		if t.Supersedes != "" {
			list = append(list, t)
		}
	}
	if len(list) == 0 {
		return nil
	}
	listings := s.listings(ctx, list)
	var out []string
	for _, t := range list {
		if listedOld(t, listings) && s.report(ctx, t, listings).State == catalog.StateOK {
			out = append(out, t.Supersedes)
		}
	}
	return out
}

func (s *Service) RemoveFormula(ctx context.Context, formula string) error {
	for _, t := range s.Placed() {
		if old, ok := t.Superseded(); ok && t.Supersedes == formula {
			return s.o.Installers[old.Method()].Remove(ctx, []catalog.Install{old}, nil)
		}
	}
	return &UnknownToolError{Name: formula}
}
