package storage

import (
	"context"
	"maps"
	"path"
	"slices"
	"sync"
	"time"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/domain/probe"
)

const (
	subject = "storage"
	pending = int64(-2)
)

type part struct {
	root int
	sign int64
}

type pathPlan struct {
	report PathReport
	parts  []part
}

type strayPlan struct {
	report StrayReport
	root   int
}

type entryPlan struct {
	entry   Entry
	paths   []pathPlan
	install []int
	strays  []strayPlan
	docker  bool
	exists  bool
}

type plan struct {
	entries []entryPlan
	roots   []string
	other   []int
}

type reading struct {
	sizes     []int64
	known     []int64
	installed map[string]bool
	listed    bool
	docker    int64
}

func (s *Service) Watch(ctx context.Context) <-chan Report {
	return probe.Stream(ctx, s.o.Pool, subject, s.watch)
}

func (s *Service) Forget() { s.o.Pool.Forget(subject) }

// invariant: the frame is every placed entry whose declared paths exist, read
// with a glob and a stat each; one walk then sizes every root of every row and
// the other caches together, and a row's size fills when its last root
// answers, its mark when its own tool answers the inventory.
func (s *Service) watch(ctx context.Context) <-chan Report {
	p := s.plan()
	frame := reading{sizes: make([]int64, len(p.roots)), known: make([]int64, len(p.roots)), docker: pending}
	known := s.loadKnown()
	for i, root := range p.roots {
		frame.sizes[i], frame.known[i] = pending, pending
		if k, ok := known[root]; ok {
			frame.known[i] = k.Bytes
		}
	}
	budget := s.walkBudget()
	jobs := []probe.Job[reading]{
		{
			Key: subject + "/walk", Lane: probe.Walks, Budget: budget,
			Partial: func(ctx context.Context, emit func(func(*reading))) func(*reading) {
				var mu sync.Mutex
				answered := slices.Repeat([]int64{Unknown}, len(p.roots))
				s.h.FS.Sizes(ctx, p.roots, func(i int, bytes int64) {
					mu.Lock()
					answered[i] = bytes
					mu.Unlock()
					emit(func(r *reading) { r.sizes[i] = bytes })
				})
				mu.Lock()
				defer mu.Unlock()
				s.saveKnown(p.roots, answered)
				return func(*reading) {}
			},
			Late: func(r *reading) {
				for i, b := range r.sizes {
					if b == pending {
						r.sizes[i] = Unknown
					}
				}
			},
		},
		{
			Key: subject + "/installed", Lane: probe.Inline,
			Partial: func(ctx context.Context, emit func(func(*reading))) func(*reading) {
				s.eachInstalled(ctx, func(installed map[string]bool) {
					emit(func(r *reading) { r.installed = installed })
				})
				return func(r *reading) { r.listed = true }
			},
			Late: func(r *reading) { r.listed = true },
		},
		{
			Key: subject + "/docker", Lane: probe.Inline,
			Do: func(ctx context.Context) func(*reading) {
				bytes := int64(0)
				if name := dockerEntry(p); name != "" && s.isInstalled(ctx, name) {
					bytes = s.dockerBytes(ctx)
				}
				return func(r *reading) { r.docker = bytes }
			},
			Late: func(r *reading) { r.docker = Unknown },
		},
	}
	readings := probe.Run(ctx, s.o.Pool, frame, jobs, func(r reading) reading {
		return reading{sizes: slices.Clone(r.sizes), known: r.known, installed: maps.Clone(r.installed), listed: r.listed, docker: r.docker}
	})
	out := make(chan Report)
	go func() {
		defer close(out)
		for r := range readings {
			select {
			case out <- s.assemble(p, r):
			case <-ctx.Done():
				return
			}
		}
	}()
	return out
}

func dockerEntry(p plan) string {
	for _, e := range p.entries {
		if e.docker {
			return e.entry.Name
		}
	}
	return ""
}

func (s *Service) plan() plan {
	var p plan
	index := map[string]int{}
	root := func(abs string) int {
		if i, ok := index[abs]; ok {
			return i
		}
		index[abs] = len(p.roots)
		p.roots = append(p.roots, abs)
		return index[abs]
	}
	glob := func(pattern string) []string {
		matches, _ := s.h.FS.Glob(s.o.Expand.Expand(pattern))
		return matches
	}
	for _, e := range s.Placed() {
		ep := entryPlan{entry: e, docker: e.Storage.Size == catalog.SizeDockerDF}
		for _, st := range e.Storage.Paths {
			if !st.PlacedOn(s.o.Here) {
				continue
			}
			pp := pathPlan{report: PathReport{Path: s.o.Expand.Display(s.o.Expand.Expand(st.Path)), Level: st.Level}}
			for _, m := range glob(st.Path) {
				pp.parts = append(pp.parts, part{root: root(m), sign: 1})
			}
			for _, memo := range st.Memos {
				for _, m := range glob(path.Join(st.Path, memo)) {
					pp.parts = append(pp.parts, part{root: root(m), sign: -1})
				}
			}
			ep.exists = ep.exists || len(pp.parts) > 0
			ep.paths = append(ep.paths, pp)
		}
		for _, pattern := range e.Storage.Installed {
			for _, m := range glob(pattern) {
				ep.install = append(ep.install, root(m))
			}
		}
		ep.exists = ep.exists || len(ep.install) > 0
		for _, st := range e.Storage.Strays {
			abs := s.o.Expand.Expand(st.Path)
			if _, err := s.h.FS.Stat(abs); err != nil {
				continue
			}
			ep.strays = append(ep.strays, strayPlan{report: StrayReport{Name: e.Name, Path: s.o.Expand.Display(abs), Repair: s.repair(e, st)}, root: root(abs)})
		}
		p.entries = append(p.entries, ep)
	}
	for _, other := range s.otherRoots() {
		p.other = append(p.other, root(other))
	}
	return p
}

func (s *Service) assemble(p plan, r reading) Report {
	report := Report{Rows: []Row{}, Strays: []StrayReport{}}
	for _, ep := range p.entries {
		row, show := s.row(ep, r)
		if !show {
			continue
		}
		report.Rows = append(report.Rows, row)
		report.Strays = append(report.Strays, row.Strays...)
	}
	for _, i := range p.other {
		if b := r.sizes[i]; b > 0 {
			report.Other += b
		}
	}
	return report
}

func (s *Service) row(ep entryPlan, r reading) (Row, bool) {
	e := ep.entry
	installed := r.installed[e.Name] || e.Name == StationName
	_, answered := r.installed[e.Name]
	marked := r.listed || answered || e.Name == StationName
	waiting := false
	var within time.Duration
	value := func(i int) int64 {
		switch b := r.sizes[i]; b {
		case pending:
			waiting = true
			return max(r.known[i], 0)
		case Unknown:
			within = max(within, s.walkBudget())
			return 0
		default:
			return b
		}
	}
	row := Row{Entry: e, Installed: installed, Bytes: map[catalog.Level]int64{}, Paths: []PathReport{}, Strays: []StrayReport{}}
	for _, pp := range ep.paths {
		report := pp.report
		for _, pt := range pp.parts {
			report.Bytes += pt.sign * value(pt.root)
		}
		row.Bytes[pp.report.Level] += report.Bytes
		row.Paths = append(row.Paths, report)
	}
	for _, i := range ep.install {
		row.Install += value(i)
	}
	for _, st := range ep.strays {
		report := st.report
		report.Bytes = value(st.root)
		row.Strays = append(row.Strays, report)
	}
	if ep.docker && installed {
		switch r.docker {
		case pending:
			waiting = true
		case Unknown:
			within = max(within, dockerDFBudget)
		default:
			row.Bytes[catalog.LevelCache] += r.docker
		}
	}
	switch {
	case len(row.Strays) > 0:
		row.State = catalog.StateDegraded
	case installed:
		row.State = catalog.StateOK
	case !marked:
		row.State = catalog.StatePending
	default:
		row.State = catalog.StateAbsent
	}
	if within > 0 && !waiting {
		unknown := s.unknown(e, installed, within)
		if !marked {
			unknown.State = catalog.StatePending
		}
		return unknown, true
	}
	if waiting {
		row.Pending = true
		return row, e.Name == StationName || ep.exists || (ep.docker && installed)
	}
	return row, e.Name == StationName || row.Total() != 0
}

func (s *Service) walkBudget() time.Duration {
	if s.o.Budget > 0 {
		return s.o.Budget
	}
	return sizeBudget
}

func (s *Service) unknown(e Entry, installed bool, within time.Duration) Row {
	row := Row{Entry: e, Installed: installed, Unknown: true, Within: within, Bytes: map[catalog.Level]int64{}, Install: Unknown, Paths: []PathReport{}, Strays: []StrayReport{}, State: catalog.StateAbsent}
	if installed {
		row.State = catalog.StateOK
	}
	for _, level := range []catalog.Level{catalog.LevelCache, catalog.LevelData, catalog.LevelConfig, catalog.LevelKept} {
		row.Bytes[level] = Unknown
	}
	for _, p := range e.Storage.Paths {
		if p.PlacedOn(s.o.Here) {
			row.Paths = append(row.Paths, PathReport{Path: s.o.Expand.Display(s.o.Expand.Expand(p.Path)), Level: p.Level, Bytes: Unknown})
		}
	}
	return row
}
