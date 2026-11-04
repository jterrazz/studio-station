package tools

import (
	"context"
	"errors"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jterrazz/studio-station/src/apps"
	"github.com/jterrazz/studio-station/src/apps/contract"
	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/domain/probe"
	"github.com/jterrazz/studio-station/src/system"
)

const (
	DetailAppStore = "app-store"
	subject        = "tools"
)

const (
	inventoryBudget = 30 * time.Second
	probeBudget     = 5 * time.Second
	applications    = "/Applications"
)

const currencyBudget = 5 * time.Second

type listing struct {
	keys     map[string]string
	currency map[string]apps.Currency
	missing  bool
	failed   bool
	unlisted bool
}

func (s *Service) Inventory(ctx context.Context) ([]Report, error) {
	return probe.Last(s.Watch(ctx)), ctx.Err()
}

func (s *Service) Watch(ctx context.Context) <-chan []Report {
	return probe.Stream(ctx, s.o.Pool, subject, s.watch)
}

func (s *Service) Forget() { s.o.Pool.Forget(subject) }

// invariant: the frame is the catalogue placed here, every row pending; one job
// per method lists its manager once (shared by every reader of the run), and
// each row fills as its own probe answers, so one hung tool holds back no other
// row of its method; a row whose method tells currency waits for it, inside
// the listing.
func (s *Service) watch(ctx context.Context) <-chan []Report {
	list := s.Placed()
	frame := make([]Report, len(list))
	byMethod := map[catalog.Method][]int{}
	var methods []catalog.Method
	for i, t := range list {
		m := s.install(t).Method()
		frame[i] = Report{Tool: t, Method: m, State: catalog.StateUnknown, Pending: true}
		if _, seen := byMethod[m]; !seen {
			methods = append(methods, m)
		}
		byMethod[m] = append(byMethod[m], i)
	}
	jobs := make([]probe.Job[[]Report], 0, len(methods))
	for _, m := range methods {
		rows := byMethod[m]
		jobs = append(jobs, probe.Job[[]Report]{
			Key: subject + "/method/" + string(m), Budget: inventoryBudget + probeBudget + currencyBudget,
			Partial: func(ctx context.Context, emit func(func(*[]Report))) func(*[]Report) {
				group := make([]catalog.Tool, len(rows))
				for k, i := range rows {
					group[k] = list[i]
				}
				each := func(k int, r Report) {
					emit(func(out *[]Report) { (*out)[rows[k]] = r })
				}
				reports := s.reportsEach(ctx, group, s.sharedListings(ctx, group), each)
				return func(out *[]Report) {
					for k, i := range rows {
						(*out)[i] = reports[k]
					}
				}
			},
			Late: func(out *[]Report) {
				for _, i := range rows {
					if !(*out)[i].Pending {
						continue
					}
					(*out)[i] = Report{Tool: list[i], Method: m, State: catalog.StateUnknown, Why: WhyLate, Detail: "no answer in " + (inventoryBudget + probeBudget + currencyBudget).String()}
				}
			},
		})
	}
	return probe.Run(ctx, s.o.Pool, frame, jobs, slices.Clone[[]Report])
}

// invariant: a shared listing is keyed by its method alone, so it asks the
// currency of every placed app of that method, whichever reader starts it.
func (s *Service) sharedListings(ctx context.Context, list []catalog.Tool) map[catalog.Method]listing {
	placed := s.Placed()
	out := map[catalog.Method]listing{}
	for m := range s.needed(list) {
		out[m] = probe.Share(ctx, s.o.Pool, subject+"/list/"+string(m), func(ctx context.Context) listing { return s.list(ctx, m, placed) })
	}
	return out
}

func (s *Service) reports(ctx context.Context, list []catalog.Tool) []Report {
	return s.reportsWith(ctx, list, s.listings(ctx, list))
}

func (s *Service) reportsWith(ctx context.Context, list []catalog.Tool, listings map[catalog.Method]listing) []Report {
	return s.reportsEach(ctx, list, listings, func(int, Report) {})
}

func (s *Service) reportsEach(ctx context.Context, list []catalog.Tool, listings map[catalog.Method]listing, each func(int, Report)) []Report {
	out := make([]Report, len(list))
	var wg sync.WaitGroup
	for i, t := range list {
		wg.Go(func() {
			out[i] = s.report(ctx, t, listings)
			each(i, out[i])
		})
	}
	wg.Wait()
	return out
}

func (s *Service) needed(list []catalog.Tool) map[catalog.Method]bool {
	needed := map[catalog.Method]bool{}
	for _, t := range list {
		needed[s.install(t).Method()] = true
		if old, ok := t.Superseded(); ok {
			needed[old.Method()] = true
		}
	}
	return needed
}

func (s *Service) listings(ctx context.Context, list []catalog.Tool) map[catalog.Method]listing {
	needed := s.needed(list)
	var mu sync.Mutex
	out := map[catalog.Method]listing{}
	var wg sync.WaitGroup
	for m := range needed {
		wg.Go(func() {
			l := s.list(ctx, m, list)
			mu.Lock()
			out[m] = l
			mu.Unlock()
		})
	}
	wg.Wait()
	return out
}

func (s *Service) list(ctx context.Context, m catalog.Method, list []catalog.Tool) listing {
	inst := s.o.Installers[m]
	l := listing{missing: inst.Ready(ctx) != nil}
	listed, cancel := context.WithTimeout(ctx, inventoryBudget)
	keys, err := inst.Inventory(listed)
	cancel()
	switch {
	case errors.Is(err, contract.ErrUnlisted):
		l.unlisted = true
		return l
	case errors.Is(err, contract.ErrMissing):
		l.missing = true
		return l
	case err != nil && l.missing:
		return l
	case err != nil:
		l.failed = true
		return l
	}
	l.keys = keys
	var held []string
	for _, t := range list {
		if i := s.install(t); i.Method() == m {
			if _, ok := keys[i.Key()]; ok {
				held = append(held, i.Key())
			}
		}
	}
	if len(held) > 0 {
		told, cancel := context.WithTimeout(ctx, currencyBudget)
		l.currency, _ = inst.Outdated(told, held)
		cancel()
	}
	return l
}

func (s *Service) report(ctx context.Context, t catalog.Tool, listings map[catalog.Method]listing) Report {
	i := s.install(t)
	m := i.Method()
	manager := s.installer(t).Manager()
	r := Report{Tool: t, Method: m, State: catalog.StateAbsent}
	l := listings[m]
	switch {
	case l.unlisted && s.bundled(t):
		r.State, r.Managed = catalog.StateOK, true
		return r
	case l.unlisted:
		return s.probed(ctx, t, r)
	case l.failed:
		r.State, r.Detail, r.Why = catalog.StateUnknown, manager.Name+" did not answer", WhyLate
		return r
	}
	if version, ok := l.keys[i.Key()]; ok {
		r.State, r.Version, r.Managed = catalog.StateOK, version, true
		// invariant: an inconclusive currency leaves the app ok, never outdated.
		switch l.currency[i.Key()] {
		case apps.Unknown:
			r.Detail = "currency unknown"
		case apps.Behind:
			r.State, r.Detail, r.Why = catalog.StateDegraded, "outdated", WhyOutdated
		case apps.Current:
		}
		return r
	}
	if s.fromAppStore(t) {
		r.State, r.Detail = catalog.StateOK, DetailAppStore
		return r
	}
	if listedOld(t, listings) && !s.bundled(t) {
		r.State, r.Detail, r.Why = catalog.StateDegraded, "formula installed, app absent", WhyPartial
		return r
	}
	if s.present(ctx, t, &r) {
		r.State = catalog.StateDegraded
		if l.missing {
			r.Detail, r.Why, r.Needs = "installed, "+manager.Name+" not installed", WhyMissing, manager.App
		} else {
			r.Detail, r.Why = "on PATH, not managed by "+string(m), WhyUnmanaged
		}
		return r
	}
	if l.missing {
		r.Detail, r.Why, r.Needs = manager.Name+" not installed", WhyMissing, manager.App
	}
	return r
}

func listedOld(t catalog.Tool, listings map[catalog.Method]listing) bool {
	old, ok := t.Superseded()
	if !ok {
		return false
	}
	_, listed := listings[old.Method()].keys[old.Key()]
	return listed
}

func (s *Service) probed(ctx context.Context, t catalog.Tool, r Report) Report {
	if t.Version.Path != "" {
		matches, _ := s.h.FS.Glob(s.home(t.Version.Path))
		if len(matches) == 0 {
			return r
		}
		r.State, r.Managed = catalog.StateOK, true
		r.Version = path.Base(matches[0])
		if len(t.Version.Args) > 0 {
			if version, code, ok := s.probe(ctx, t.Version); ok && code == 0 {
				r.Version = version
			}
		}
		return r
	}
	if !s.provided(t) {
		return r
	}
	if len(t.Version.Args) == 0 {
		r.State, r.Managed = catalog.StateOK, true
		return r
	}
	version, code, ok := s.probe(ctx, t.Version)
	switch {
	case ok && code == 0:
		r.State, r.Version, r.Managed = catalog.StateOK, version, true
	case ok:
		r.State, r.Why, r.Detail = catalog.StateBroken, WhyFailing, strings.Join(t.Version.Args, " ")+" exited "+strconv.Itoa(code)
	}
	return r
}

func (s *Service) present(ctx context.Context, t catalog.Tool, r *Report) bool {
	if s.bundled(t) {
		return true
	}
	for _, name := range t.Provides {
		where, ok := s.h.Run.LookPath(name)
		if !ok {
			continue
		}
		// hazard: stock macOS ships /usr/bin shims (java, python3, xcodebuild, git)
		// that exist without the tool; such a hit counts only when the probe answers.
		if strings.HasPrefix(where, "/usr/bin/") {
			if len(t.Version.Args) == 0 {
				continue
			}
			version, code, ok := s.probe(ctx, t.Version)
			if !ok || code != 0 {
				continue
			}
			r.Version = version
			return true
		}
		if len(t.Version.Args) > 0 {
			if version, code, ok := s.probe(ctx, t.Version); ok && code == 0 {
				r.Version = version
			}
		}
		return true
	}
	return false
}

func (s *Service) provided(t catalog.Tool) bool {
	for _, name := range t.Provides {
		if _, ok := s.h.Run.LookPath(name); ok {
			return true
		}
	}
	return len(t.Provides) == 0 && len(t.Version.Args) > 0
}

func (s *Service) bundled(t catalog.Tool) bool {
	if t.Bundle == "" {
		return false
	}
	_, err := s.h.FS.Stat(path.Join(s.o.Root, applications, t.Bundle+".app"))
	return err == nil
}

// invariant: an App Store copy of an AppStore tool's bundle is its install,
// not station's: present, never managed, upgraded or removed by station.
func (s *Service) fromAppStore(t catalog.Tool) bool {
	if !t.AppStore || t.Bundle == "" {
		return false
	}
	_, err := s.h.FS.Stat(path.Join(s.o.Root, applications, t.Bundle+".app", catalog.AppStoreReceipt))
	return err == nil
}

func (s *Service) probe(ctx context.Context, p catalog.Probe) (version string, code int, ran bool) {
	args := make([]string, 0, len(p.Args)-1)
	for _, a := range p.Args[1:] {
		args = append(args, s.home(a))
	}
	out, err := s.h.Run.Output(ctx, system.Cmd{Name: p.Args[0], Args: args, Timeout: probeBudget})
	if err != nil {
		return "", 0, false
	}
	text := out.Stdout + out.Stderr
	if p.Pattern == "" {
		return strings.TrimSpace(text), out.ExitCode, true
	}
	re, err := regexp.Compile("(?m)" + p.Pattern)
	if err != nil {
		return "", out.ExitCode, true
	}
	if m := re.FindStringSubmatch(text); len(m) > 1 {
		return m[1], out.ExitCode, true
	}
	return "", out.ExitCode, true
}

func (s *Service) home(p string) string {
	if p == "~" || strings.HasPrefix(p, "~/") {
		return s.h.Env.Home() + p[1:]
	}
	return p
}
