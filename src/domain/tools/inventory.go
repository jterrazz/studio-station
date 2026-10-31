package tools

import (
	"context"
	"path"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/system"
)

const (
	inventoryBudget = 30 * time.Second
	probeBudget     = 5 * time.Second
	applications    = "/Applications"
)

type listing struct {
	keys    map[string]string
	missing bool
	failed  bool
	skills  map[string]lockEntry
}

func (s *Service) Inventory(ctx context.Context) ([]Report, error) {
	return s.reports(ctx, s.Placed()), nil
}

func (s *Service) reports(ctx context.Context, list []catalog.Tool) []Report {
	listings := s.listings(ctx, list)
	out := make([]Report, len(list))
	var wg sync.WaitGroup
	for i, t := range list {
		wg.Go(func() { out[i] = s.report(ctx, t, listings) })
	}
	wg.Wait()
	s.currency(ctx, out, listings[catalog.MethodSkill])
	return out
}

func (s *Service) listings(ctx context.Context, list []catalog.Tool) map[catalog.Method]listing {
	needed := map[catalog.Method]bool{}
	for _, t := range list {
		needed[s.install(t).Method()] = true
	}
	var mu sync.Mutex
	out := map[catalog.Method]listing{}
	var wg sync.WaitGroup
	for m := range needed {
		wg.Go(func() {
			l := s.list(ctx, m)
			mu.Lock()
			out[m] = l
			mu.Unlock()
		})
	}
	wg.Wait()
	return out
}

func (s *Service) list(ctx context.Context, m catalog.Method) listing {
	if m == catalog.MethodSkill {
		return listing{skills: s.readLock()}
	}
	row := s.table[m]
	if len(row.inventory) == 0 {
		return listing{}
	}
	if _, ok := s.h.Run.LookPath(row.manager); !ok {
		return listing{missing: true}
	}
	c := row.inventory[0]
	c.Timeout = inventoryBudget
	out, err := s.h.Run.Output(ctx, c)
	if err != nil || (out.ExitCode != 0 && !row.anyExit) {
		return listing{failed: true}
	}
	keys, err := row.parse(out.Stdout)
	if err != nil {
		return listing{failed: true}
	}
	return listing{keys: keys}
}

func (s *Service) report(ctx context.Context, t catalog.Tool, listings map[catalog.Method]listing) Report {
	i := s.install(t)
	m := i.Method()
	row := s.table[m]
	r := Report{Tool: t, Method: m, State: catalog.StateAbsent}
	l := listings[m]
	switch {
	case m == catalog.MethodSkill:
		if _, ok := l.skills[i.Key()]; ok {
			r.State, r.Managed = catalog.StateOK, true
			return r
		}
		if _, ok := s.h.Run.LookPath("skills"); !ok {
			r.Detail = "skills not installed"
		}
		return r
	case m == catalog.MethodScript || m == catalog.MethodShell:
		return s.probed(ctx, t, r)
	case m == catalog.MethodApp:
		if s.bundled(t) {
			r.State, r.Managed = catalog.StateOK, true
		}
		return r
	case l.failed:
		r.State, r.Detail = catalog.StateUnknown, row.manager+" did not answer"
		return r
	}
	if version, ok := l.keys[i.Key()]; ok {
		r.State, r.Version, r.Managed = catalog.StateOK, version, true
		return r
	}
	if s.present(ctx, t, &r) {
		r.State = catalog.StateDegraded
		if l.missing {
			r.Detail = "installed, " + row.manager + " not installed"
		} else {
			r.Detail = "on PATH, not managed by " + string(m)
		}
		return r
	}
	if l.missing {
		r.Detail = row.manager + " not installed"
	}
	return r
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
		r.State, r.Detail = catalog.StateBroken, strings.Join(t.Version.Args, " ")+" exited "+strconv.Itoa(code)
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
	_, err := s.h.FS.Stat(path.Join(applications, t.Bundle+".app"))
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
