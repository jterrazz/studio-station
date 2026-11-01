package storage

import (
	"context"
	"errors"
	"io/fs"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/system"
)

const (
	StationName     = "station"
	StationCategory = "Station"
	dockerDFBudget  = 5 * time.Second
	quitBudget      = 2 * time.Second
	sizeBudget      = 10 * time.Second
	Unknown         = int64(-1)
)

type Inventory interface {
	Installed(ctx context.Context) (map[string]bool, error)
}

type Options struct {
	Here      catalog.MachineKind
	Expand    Expander
	Files     map[catalog.ToolFile]string
	Inventory Inventory
	Budget    time.Duration
}

type Service struct {
	h     system.Host
	tools []catalog.Tool
	o     Options
}

func New(h system.Host, tools []catalog.Tool, o Options) *Service {
	return &Service{h: h, tools: tools, o: o}
}

type Entry struct {
	Name     string
	Category string
	Storage  catalog.Storage
	Places   []catalog.MachineKind
}

type PathReport struct {
	Path  string
	Level catalog.Level
	Bytes int64
}

type StrayReport struct {
	Name   string
	Path   string
	Bytes  int64
	Repair string
}

type Row struct {
	Entry     Entry
	Installed bool
	Unknown   bool
	State     catalog.State
	Bytes     map[catalog.Level]int64
	Install   int64
	Paths     []PathReport
	Strays    []StrayReport
}

func (r Row) Total() int64 {
	total := r.Install
	for _, b := range r.Bytes {
		total += b
	}
	for _, s := range r.Strays {
		total += s.Bytes
	}
	return total
}

type Report struct {
	Rows   []Row
	Strays []StrayReport
	Other  int64
}

type Outcome struct {
	Name   string
	Result catalog.State
	Detail string
	Reason string
	Hint   string
	Freed  int64
	Reset  bool
}

func (s *Service) Entries() []Entry {
	out := []Entry{{Name: StationName, Category: StationCategory, Storage: catalog.StationStorage, Places: []catalog.MachineKind{catalog.Host, catalog.Linux, catalog.MacOS}}}
	for _, t := range s.tools {
		if t.Storage.Declared() {
			out = append(out, Entry{Name: t.Name, Category: string(t.Category), Storage: t.Storage, Places: t.Places()})
		}
	}
	return out
}

func (s *Service) Placed() []Entry {
	var out []Entry
	for _, e := range s.Entries() {
		if s.placed(e) {
			out = append(out, e)
		}
	}
	return out
}

func (s *Service) Names() []string {
	placed := s.Placed()
	out := make([]string, 0, len(placed))
	for _, e := range placed {
		out = append(out, e.Name)
	}
	return out
}

func (s *Service) placed(e Entry) bool {
	for _, k := range e.Places {
		if k == s.o.Here {
			return true
		}
	}
	return false
}

func (s *Service) find(name string) (Entry, error) {
	for _, e := range s.Entries() {
		if e.Name != name {
			continue
		}
		if !s.placed(e) {
			return e, &WrongPlaceError{Name: name, Places: e.Places, Here: s.o.Here}
		}
		return e, nil
	}
	return Entry{}, &UnknownToolError{Name: name}
}

func (s *Service) installed(ctx context.Context) map[string]bool {
	out := map[string]bool{}
	if s.o.Inventory != nil {
		if got, err := s.o.Inventory.Installed(ctx); err == nil {
			out = got
		}
	}
	out[StationName] = true
	return out
}

func (s *Service) List(ctx context.Context) (Report, error) {
	installed := s.installed(ctx)
	report := Report{Rows: []Row{}, Strays: []StrayReport{}}
	for _, row := range s.measureAll(ctx, s.Placed(), installed) {
		if row.Entry.Name != StationName && !row.Unknown && row.Total() == 0 {
			continue
		}
		report.Rows = append(report.Rows, row)
		report.Strays = append(report.Strays, row.Strays...)
	}
	report.Other = s.other()
	return report, nil
}

func (s *Service) other() int64 {
	var claimed []string
	for _, e := range s.Placed() {
		for _, p := range e.Storage.Paths {
			claimed = append(claimed, s.o.Expand.Expand(p.Path))
		}
		for _, p := range e.Storage.Installed {
			claimed = append(claimed, s.o.Expand.Expand(p))
		}
	}
	roots := []string{s.o.Expand.Roots.Cache}
	if s.o.Here != catalog.Linux {
		roots = append(roots, path.Join(s.o.Expand.Home, "Library", "Caches"))
	}
	var total int64
	for _, root := range roots {
		entries, err := s.h.FS.ReadDir(root)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if strings.HasPrefix(entry.Name(), "com.apple.") {
				continue
			}
			full := path.Join(root, entry.Name())
			if !claims(claimed, full) {
				total += s.h.FS.DirSize(full)
			}
		}
	}
	return total
}

func claims(patterns []string, entry string) bool {
	for _, p := range patterns {
		if p == entry || strings.HasPrefix(p, entry+"/") || strings.HasPrefix(entry, strings.TrimSuffix(p, "/*")+"/") {
			return true
		}
		if ok, _ := path.Match(p, entry); ok {
			return true
		}
	}
	return false
}

// invariant: one slow tree (a simulator runtime, /opt/homebrew) never holds the list
// past the budget: its row answers Unknown and the others keep their sizes.
func (s *Service) measureAll(ctx context.Context, entries []Entry, installed map[string]bool) []Row {
	budget := s.o.Budget
	if budget == 0 {
		budget = sizeBudget
	}
	ctx, cancel := context.WithTimeout(ctx, budget)
	defer cancel()
	done := make([]chan Row, len(entries))
	for i, e := range entries {
		done[i] = make(chan Row, 1)
		go func() { done[i] <- s.measure(ctx, e, installed[e.Name]) }()
	}
	rows := make([]Row, len(entries))
	for i, e := range entries {
		select {
		case rows[i] = <-done[i]:
		case <-ctx.Done():
			select {
			case rows[i] = <-done[i]:
			default:
				rows[i] = s.unknown(e, installed[e.Name])
			}
		}
	}
	return rows
}

func (s *Service) unknown(e Entry, installed bool) Row {
	row := Row{Entry: e, Installed: installed, Unknown: true, Bytes: map[catalog.Level]int64{}, Install: Unknown, Paths: []PathReport{}, Strays: []StrayReport{}, State: catalog.StateAbsent}
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

func (s *Service) measure(ctx context.Context, e Entry, installed bool) Row {
	row := Row{Entry: e, Installed: installed, Bytes: map[catalog.Level]int64{}, Paths: []PathReport{}, Strays: []StrayReport{}}
	for _, p := range e.Storage.Paths {
		if !p.PlacedOn(s.o.Here) {
			continue
		}
		bytes := s.storedBytes(p)
		row.Bytes[p.Level] += bytes
		row.Paths = append(row.Paths, PathReport{Path: s.o.Expand.Display(s.o.Expand.Expand(p.Path)), Level: p.Level, Bytes: bytes})
	}
	if e.Storage.Size == catalog.SizeDockerDF && installed {
		row.Bytes[catalog.LevelCache] += s.dockerBytes(ctx)
	}
	for _, p := range e.Storage.Installed {
		row.Install += s.size(p)
	}
	row.Strays = s.strays(e)
	switch {
	case len(row.Strays) > 0:
		row.State = catalog.StateDegraded
	case installed:
		row.State = catalog.StateOK
	default:
		row.State = catalog.StateAbsent
	}
	return row
}

func (s *Service) size(pattern string) int64 {
	matches, _ := s.h.FS.Glob(s.o.Expand.Expand(pattern))
	var total int64
	for _, m := range matches {
		total += s.h.FS.DirSize(m)
	}
	return total
}

func (s *Service) exists(pattern string) bool {
	matches, _ := s.h.FS.Glob(s.o.Expand.Expand(pattern))
	return len(matches) > 0
}

func (s *Service) strays(e Entry) []StrayReport {
	out := []StrayReport{}
	for _, st := range e.Storage.Strays {
		abs := s.o.Expand.Expand(st.Path)
		if _, err := s.h.FS.Stat(abs); err != nil {
			continue
		}
		repair := "station storage clean " + e.Name
		if s.movable(st) || !s.Relocated(e.Storage) {
			repair = "station setup"
		}
		out = append(out, StrayReport{Name: e.Name, Path: s.o.Expand.Display(abs), Bytes: s.h.FS.DirSize(abs), Repair: repair})
	}
	return out
}

func (s *Service) movable(st catalog.Stray) bool {
	for _, m := range st.Move {
		if s.exists(m.From) {
			return true
		}
	}
	return false
}

func (s *Service) Relocated(st catalog.Storage) bool {
	for _, r := range st.Relocate {
		if r.Pending != "" {
			continue
		}
		file := s.o.Files[r.File]
		if file == "" {
			continue
		}
		data, err := s.h.FS.ReadFile(file)
		if err != nil {
			return false
		}
		if _, ok := LineValue(string(data), r.Key); !ok {
			return false
		}
	}
	return true
}

func (s *Service) Clean(ctx context.Context, names []string) ([]Outcome, error) {
	var entries []Entry
	named := len(names) > 0
	for _, name := range dedupe(names) {
		e, err := s.find(name)
		if err != nil {
			return nil, err
		}
		if len(e.Storage.Clean) == 0 && len(e.Storage.Strays) == 0 {
			return nil, &NoCleanError{Name: name}
		}
		entries = append(entries, e)
	}
	installed := s.installed(ctx)
	if !named {
		for _, e := range s.Placed() {
			if len(e.Storage.Clean) == 0 && len(e.Storage.Strays) == 0 {
				continue
			}
			if installed[e.Name] || s.cacheBytes(e) > 0 || len(s.strays(e)) > 0 {
				entries = append(entries, e)
			}
		}
	}
	outcomes := make([]Outcome, 0, len(entries))
	for _, e := range entries {
		present := installed[e.Name]
		before := s.cacheBytes(e)
		if !present && before == 0 && len(s.strays(e)) == 0 {
			outcomes = append(outcomes, Outcome{Name: e.Name, Result: catalog.StateAbsent, Detail: "not installed"})
			continue
		}
		failure := s.actions(ctx, e.Storage.Clean, present)
		if !present {
			failure = first(failure, s.deleteCaches(e))
		}
		for _, st := range e.Storage.Strays {
			if _, err := s.h.FS.Stat(s.o.Expand.Expand(st.Path)); err == nil {
				failure = first(failure, s.actions(ctx, st.Clean, present))
			}
		}
		outcomes = append(outcomes, s.outcome(e.Name, failure, before-s.cacheBytes(e), false))
	}
	return outcomes, nil
}

func (s *Service) CheckReset(names []string) ([]Entry, error) {
	if len(names) == 0 {
		return nil, &NoNameError{}
	}
	var entries []Entry
	for _, name := range dedupe(names) {
		e, err := s.find(name)
		if err != nil {
			return nil, err
		}
		if len(e.Storage.Reset) == 0 {
			return nil, &NoResetError{Name: name, Hint: e.Storage.NoReset}
		}
		entries = append(entries, e)
	}
	return entries, nil
}

func (s *Service) Plan(names []string) []PathReport {
	entries, _ := s.CheckReset(names)
	var out []PathReport
	for _, e := range entries {
		for _, p := range e.Storage.Paths {
			if !p.PlacedOn(s.o.Here) || (p.Level != catalog.LevelCache && p.Level != catalog.LevelData) {
				continue
			}
			out = append(out, PathReport{Path: s.o.Expand.Display(s.o.Expand.Expand(p.Path)), Level: p.Level, Bytes: s.storedBytes(p)})
		}
	}
	return out
}

func (s *Service) Reset(ctx context.Context, names []string) ([]Outcome, error) {
	entries, err := s.CheckReset(names)
	if err != nil {
		return nil, err
	}
	installed := s.installed(ctx)
	outcomes := make([]Outcome, 0, len(entries))
	for _, e := range entries {
		if e.Storage.Quit != "" && s.running(ctx, e.Storage.Quit) {
			outcomes = append(outcomes, Outcome{
				Name: e.Name, Result: catalog.StateBroken, Detail: e.Storage.Quit + " is running",
				Hint: "quit " + e.Storage.Quit + ", then station storage reset " + e.Name,
			})
			continue
		}
		before := s.levelBytes(e, catalog.LevelCache) + s.levelBytes(e, catalog.LevelData)
		present := installed[e.Name]
		failure := first(s.actions(ctx, e.Storage.Clean, present), s.actions(ctx, e.Storage.Reset, present))
		if !present {
			failure = first(failure, s.deleteCaches(e))
		}
		after := s.levelBytes(e, catalog.LevelCache) + s.levelBytes(e, catalog.LevelData)
		outcomes = append(outcomes, s.outcome(e.Name, failure, before-after, true))
	}
	return outcomes, nil
}

func (s *Service) outcome(name string, f *actionFailure, freed int64, reset bool) Outcome {
	if f != nil {
		return Outcome{Name: name, Result: catalog.StateBroken, Detail: f.detail, Reason: f.reason, Reset: reset}
	}
	return Outcome{Name: name, Result: catalog.StateOK, Freed: max(freed, 0), Reset: reset}
}

func (s *Service) running(ctx context.Context, process string) bool {
	out, err := s.h.Run.Output(ctx, system.Cmd{Name: "pgrep", Args: []string{"-x", process}, Timeout: quitBudget})
	return err == nil && out.ExitCode == 0
}

func (s *Service) cacheBytes(e Entry) int64 {
	total := s.levelBytes(e, catalog.LevelCache)
	for _, st := range e.Storage.Strays {
		total += s.h.FS.DirSize(s.o.Expand.Expand(st.Path))
	}
	return total
}

func (s *Service) levelBytes(e Entry, level catalog.Level) int64 {
	var total int64
	for _, p := range e.Storage.Paths {
		if p.Level == level && p.PlacedOn(s.o.Here) {
			total += s.storedBytes(p)
		}
	}
	return total
}

// invariant: a memo a tool rewrites on every call (mise's bin_paths, written by each
// shim and by station's own reads) is not counted, so a cleaned cache measures empty.
func (s *Service) storedBytes(st catalog.Stored) int64 {
	total := s.size(st.Path)
	for _, m := range st.Memos {
		total -= s.size(path.Join(st.Path, m))
	}
	return total
}

type actionFailure struct{ detail, reason string }

func first(a, b *actionFailure) *actionFailure {
	if a != nil {
		return a
	}
	return b
}

// invariant: every action runs even after one failed, and the first failure is the one
// reported, so a stopped docker daemon does not hide an image prune that could run.
// A tool that is not installed has no binary to run, so only its plain deletions run.
func (s *Service) actions(ctx context.Context, actions []catalog.Action, present bool) *actionFailure {
	var failed *actionFailure
	for _, a := range actions {
		if !a.PlacedOn(s.o.Here) || (!present && len(a.Run) > 0) {
			continue
		}
		failed = first(failed, s.action(ctx, a))
	}
	return failed
}

func (s *Service) action(ctx context.Context, a catalog.Action) *actionFailure {
	if a.Needs != "" {
		if _, ok := s.h.Run.LookPath(a.Needs); !ok {
			return nil
		}
	}
	if a.Exists != "" && !s.exists(a.Exists) {
		return nil
	}
	var failed *actionFailure
	if len(a.Run) > 0 {
		failed = s.run(ctx, a)
	}
	for _, pattern := range a.Delete {
		failed = first(failed, s.delete(pattern))
	}
	for _, dir := range a.Prune {
		_ = s.h.FS.Remove(s.o.Expand.Expand(dir))
	}
	return failed
}

func (s *Service) deleteCaches(e Entry) *actionFailure {
	var failed *actionFailure
	for _, p := range e.Storage.Paths {
		if p.Level == catalog.LevelCache && p.PlacedOn(s.o.Here) {
			failed = first(failed, s.delete(p.Path))
		}
	}
	return failed
}

// invariant: a tree the tool wrote read-only (Go's module cache) is made writable and
// removed again, since without the tool's own clean nothing else can remove it.
func (s *Service) delete(pattern string) *actionFailure {
	matches, _ := s.h.FS.Glob(s.o.Expand.Expand(pattern))
	var failed *actionFailure
	for _, m := range matches {
		if s.h.FS.RemoveAll(m) == nil {
			continue
		}
		_ = s.h.FS.WalkDir(m, func(p string, d fs.DirEntry, err error) error {
			if err == nil && d.IsDir() {
				_ = s.h.FS.Chmod(p, 0o700)
			}
			return nil
		})
		if err := s.h.FS.RemoveAll(m); err != nil {
			failed = first(failed, &actionFailure{detail: err.Error()})
		}
	}
	return failed
}

func (s *Service) run(ctx context.Context, a catalog.Action) *actionFailure {
	args := make([]string, 0, len(a.Run)-1)
	for _, w := range a.Run[1:] {
		args = append(args, s.o.Expand.Word(w))
	}
	env := make([]string, 0, len(a.Env))
	for _, kv := range a.Env {
		env = append(env, s.o.Expand.Word(kv))
	}
	label := label(a.Run)
	ran, err := s.h.Run.Attach(ctx, system.Cmd{Name: a.Run[0], Args: args, Env: env}, system.ToStderr)
	if errors.Is(err, system.ErrNotFound) {
		return nil
	}
	if err != nil {
		return &actionFailure{detail: label + ": " + err.Error()}
	}
	if ran.ExitCode != 0 {
		return &actionFailure{detail: label + " exited " + strconv.Itoa(ran.ExitCode), reason: ran.LastStderr}
	}
	return nil
}

func label(argv []string) string {
	words := []string{argv[0]}
	for _, w := range argv[1:] {
		if strings.HasPrefix(w, "-") {
			break
		}
		words = append(words, w)
	}
	return strings.Join(words, " ")
}

func (s *Service) dockerBytes(ctx context.Context) int64 {
	out, err := s.h.Run.Output(ctx, system.Cmd{Name: "docker", Args: []string{"system", "df", "--format", "{{.Size}}"}, Timeout: dockerDFBudget})
	if err != nil || out.ExitCode != 0 {
		return 0
	}
	var sum int64
	for _, line := range strings.Split(out.Stdout, "\n") {
		sum += dockerSize(strings.TrimSpace(line))
	}
	return sum
}

// invariant: docker prints decimal units (kB, MB, GB, TB), the same ones station prints.
func dockerSize(s string) int64 {
	for _, unit := range []struct {
		suffix string
		scale  float64
	}{{"TB", 1e12}, {"GB", 1e9}, {"MB", 1e6}, {"kB", 1e3}, {"B", 1}} {
		if number, ok := strings.CutSuffix(s, unit.suffix); ok {
			value, err := strconv.ParseFloat(number, 64)
			if err != nil {
				return 0
			}
			return int64(value * unit.scale)
		}
	}
	return 0
}

func dedupe(names []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, n := range names {
		if !seen[n] {
			seen[n] = true
			out = append(out, n)
		}
	}
	return out
}

func Failures(outcomes []Outcome) (failed int, firstFailed Outcome) {
	for _, o := range outcomes {
		if o.Result != catalog.StateBroken {
			continue
		}
		if failed == 0 {
			firstFailed = o
		}
		failed++
	}
	return failed, firstFailed
}
