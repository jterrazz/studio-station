package clean

import (
	"context"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/system"
)

const dockerDFBudget = 5 * time.Second

type Reclaimable struct {
	Name  string
	Bytes int64
}

type StaleVMs interface {
	List(ctx context.Context) ([]Reclaimable, error)
	Remove(ctx context.Context, name string) error
}

type OldImages interface {
	List(ctx context.Context) ([]Reclaimable, error)
	Remove(ctx context.Context, name string) error
}

type Options struct {
	Here      catalog.MachineKind
	StaleVMs  StaleVMs
	OldImages OldImages
}

type Service struct {
	h      system.Host
	items  []catalog.Cleanable
	byName map[string]catalog.Cleanable
	o      Options
}

func New(h system.Host, items []catalog.Cleanable, o Options) *Service {
	byName := make(map[string]catalog.Cleanable, len(items))
	for _, c := range items {
		byName[c.Name] = c
	}
	return &Service{h: h, items: items, byName: byName, o: o}
}

type Report struct {
	Item      catalog.Cleanable
	Available bool
	Bytes     int64
}

type Outcome struct {
	Name   string
	Result catalog.State
	Detail string
}

func (s *Service) Placed() []catalog.Cleanable {
	var placed []catalog.Cleanable
	for _, c := range s.items {
		if slices.Contains(c.Places, s.o.Here) {
			placed = append(placed, c)
		}
	}
	return placed
}

func (s *Service) Find(names []string) ([]catalog.Cleanable, error) {
	seen := map[string]bool{}
	var found []catalog.Cleanable
	for _, name := range names {
		if seen[name] {
			continue
		}
		seen[name] = true
		c, ok := s.byName[name]
		if !ok {
			return nil, &UnknownCleanableError{Name: name}
		}
		if !slices.Contains(c.Places, s.o.Here) {
			return nil, &WrongPlaceError{Name: name, Places: c.Places, Here: s.o.Here}
		}
		found = append(found, c)
	}
	return found, nil
}

func (s *Service) List(ctx context.Context) ([]Report, error) {
	placed := s.Placed()
	reports := make([]Report, 0, len(placed))
	for _, c := range placed {
		available, bytes := s.measure(ctx, c)
		reports = append(reports, Report{Item: c, Available: available, Bytes: bytes})
	}
	return reports, nil
}

func (s *Service) Clean(ctx context.Context, names []string) ([]Outcome, error) {
	found, err := s.Find(names)
	if err != nil {
		return nil, err
	}
	outcomes := make([]Outcome, 0, len(found))
	for _, c := range found {
		outcomes = append(outcomes, s.clean(ctx, c))
	}
	return outcomes, nil
}

func (s *Service) clean(ctx context.Context, c catalog.Cleanable) Outcome {
	if c.Requires != "" {
		if _, ok := s.h.Run.LookPath(c.Requires); !ok {
			return Outcome{Name: c.Name, Result: catalog.StateAbsent, Detail: c.Requires + " not found"}
		}
	}
	switch c.Size {
	case catalog.SizeVMs:
		return s.drain(ctx, c, s.o.StaleVMs)
	case catalog.SizeImages:
		return s.drain(ctx, c, s.o.OldImages)
	case catalog.SizeDirs, catalog.SizeDockerDF:
	}
	if detail, err := s.steps(ctx, c); err != nil {
		return Outcome{Name: c.Name, Result: catalog.StateBroken, Detail: detail}
	}
	for _, dir := range c.EmptyDirs {
		dir = s.home(dir)
		if err := s.h.FS.RemoveAll(dir); err != nil {
			return Outcome{Name: c.Name, Result: catalog.StateBroken, Detail: err.Error()}
		}
		if err := s.h.FS.MkdirAll(dir, 0o700); err != nil {
			return Outcome{Name: c.Name, Result: catalog.StateBroken, Detail: err.Error()}
		}
	}
	return Outcome{Name: c.Name, Result: catalog.StateOK, Detail: "cleaned"}
}

// invariant: every step runs even after one failed, and the first failure is the one
// reported, so a stopped docker daemon does not hide an image prune that could run.
func (s *Service) steps(ctx context.Context, c catalog.Cleanable) (string, error) {
	var first error
	detail := ""
	for _, step := range c.Steps {
		ran, err := s.h.Run.Attach(ctx, system.Cmd{Name: step[0], Args: step[1:]}, system.ToStderr)
		code := ran.ExitCode
		if err == nil && code != 0 {
			err = &stepError{step: strings.Join(step, " "), code: code}
		}
		if err != nil && first == nil {
			first = err
			detail = err.Error()
		}
	}
	return detail, first
}

type stepError struct {
	step string
	code int
}

func (e *stepError) Error() string { return e.step + " exited " + strconv.Itoa(e.code) }

type source interface {
	List(ctx context.Context) ([]Reclaimable, error)
	Remove(ctx context.Context, name string) error
}

func (s *Service) drain(ctx context.Context, c catalog.Cleanable, src source) Outcome {
	if src == nil {
		return Outcome{Name: c.Name, Result: catalog.StateAbsent, Detail: "not available here"}
	}
	list, err := src.List(ctx)
	if err != nil {
		return Outcome{Name: c.Name, Result: catalog.StateBroken, Detail: err.Error()}
	}
	if len(list) == 0 {
		return Outcome{Name: c.Name, Result: catalog.StateOK, Detail: "nothing to reclaim"}
	}
	for i, r := range list {
		if err := src.Remove(ctx, r.Name); err != nil {
			return Outcome{Name: c.Name, Result: catalog.StateBroken, Detail: r.Name + ": " + err.Error() + " (" + strconv.Itoa(i) + " of " + strconv.Itoa(len(list)) + " removed)"}
		}
	}
	return Outcome{Name: c.Name, Result: catalog.StateOK, Detail: strconv.Itoa(len(list)) + " removed"}
}

func (s *Service) measure(ctx context.Context, c catalog.Cleanable) (available bool, bytes int64) {
	if c.Requires != "" {
		if _, ok := s.h.Run.LookPath(c.Requires); !ok {
			return false, 0
		}
	}
	switch c.Size {
	case catalog.SizeDirs:
		var total int64
		for _, dir := range c.Dirs {
			total += s.h.FS.DirSize(s.home(dir))
		}
		return true, total
	case catalog.SizeDockerDF:
		return s.dockerBytes(ctx)
	case catalog.SizeVMs:
		return sumOf(ctx, s.o.StaleVMs)
	case catalog.SizeImages:
		return sumOf(ctx, s.o.OldImages)
	}
	return true, 0
}

func sumOf(ctx context.Context, src source) (available bool, bytes int64) {
	if src == nil {
		return false, 0
	}
	list, err := src.List(ctx)
	if err != nil {
		return false, 0
	}
	var sum int64
	for _, r := range list {
		sum += r.Bytes
	}
	return true, sum
}

func (s *Service) home(p string) string {
	if p == "~" || strings.HasPrefix(p, "~/") {
		return s.h.Env.Home() + p[1:]
	}
	return p
}
