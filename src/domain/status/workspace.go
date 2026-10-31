package status

import (
	"context"
	"path"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/system"
)

// value: depth 2 covers ~/Developer/<org>/<repo> and keeps the scan fast on a
// machine with deep dependency trees.
const repoDepth = 2

func (s *Service) workspace(ctx context.Context) Workspace {
	var (
		ws Workspace
		wg sync.WaitGroup
	)
	run := func(fn func()) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			fn()
		}()
	}
	root := path.Join(s.h.Env.Home(), "Developer")
	paths := s.findRepos(root, 0)
	run(func() { ws.Repositories = s.repositories(ctx, root, paths) })
	run(func() { ws.Dependencies = s.dependencies(ctx, root, paths) })
	run(func() { ws.Docker = s.docker(ctx) })
	run(func() { ws.Disk = s.disk() })
	wg.Wait()
	if ws.Docker.TotalBytes > 0 {
		ws.Disk = append([]Disk{{Name: "docker", Bytes: ws.Docker.TotalBytes}}, ws.Disk...)
	}
	return ws
}

func (s *Service) findRepos(dir string, depth int) []string {
	if depth > repoDepth {
		return nil
	}
	if _, err := s.h.FS.Stat(path.Join(dir, ".git")); err == nil {
		return []string{dir}
	}
	entries, err := s.h.FS.ReadDir(dir)
	if err != nil {
		return nil
	}
	var found []string
	for _, e := range entries {
		if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
			found = append(found, s.findRepos(path.Join(dir, e.Name()), depth+1)...)
		}
	}
	return found
}

func (s *Service) repositories(ctx context.Context, root string, paths []string) []Repository {
	repos := make([]Repository, len(paths))
	var wg sync.WaitGroup
	for i, p := range paths {
		wg.Add(1)
		go func() {
			defer wg.Done()
			repos[i] = scanRepo(strings.TrimPrefix(p, root+"/"), s.ask(ctx, system.Cmd{Name: "git", Args: []string{"-C", p, "status", "--porcelain", "-b"}}, defaultBudget))
		}()
	}
	wg.Wait()
	sort.Slice(repos, func(i, j int) bool { return repos[i].Path < repos[j].Path })
	if repos == nil {
		repos = []Repository{}
	}
	return repos
}

// outside: `git status --porcelain -b` opens with `## main...origin/main [ahead 1]`,
// `## HEAD (no branch)` or `## No commits yet on main`, then one line per change.
func scanRepo(rel string, r reply) Repository {
	repo := Repository{Path: rel, Branch: "?"}
	if !r.ok() {
		return repo
	}
	lines := strings.Split(strings.TrimRight(r.text(), "\n"), "\n")
	if head, ok := strings.CutPrefix(lines[0], "## "); ok {
		head = strings.TrimPrefix(head, "No commits yet on ")
		head, _, _ = strings.Cut(head, "...")
		head, _, _ = strings.Cut(head, " ")
		repo.Branch = head
		lines = lines[1:]
	}
	for _, l := range lines {
		if strings.TrimSpace(l) != "" {
			repo.Dirty++
		}
	}
	return repo
}

func (s *Service) manager(dir string) string {
	for _, m := range [][2]string{{"pnpm-lock.yaml", "pnpm"}, {"go.sum", "go"}, {"bun.lock", "bun"}, {"package-lock.json", "npm"}} {
		if _, err := s.h.FS.Stat(path.Join(dir, m[0])); err == nil {
			return m[1]
		}
	}
	return ""
}

// invariant: a repository whose scan outlives its budget is listed with outdated -1,
// never dropped and never counted as current.
func (s *Service) dependencies(ctx context.Context, root string, paths []string) []Dependency {
	ctx, cancel := context.WithTimeout(ctx, scanBudget)
	defer cancel()
	deps := make([]Dependency, 0, len(paths))
	managed := make([]string, 0, len(paths))
	for _, p := range paths {
		if m := s.manager(p); m != "" {
			deps = append(deps, Dependency{Path: strings.TrimPrefix(p, root+"/"), Manager: m})
			managed = append(managed, p)
		}
	}
	keep := make([]bool, len(deps))
	var wg sync.WaitGroup
	for i := range deps {
		wg.Add(1)
		go func() {
			defer wg.Done()
			deps[i].Outdated, keep[i] = s.outdated(ctx, managed[i], deps[i].Manager)
		}()
	}
	wg.Wait()
	scanned := deps[:0]
	for i, d := range deps {
		if keep[i] {
			scanned = append(scanned, d)
		}
	}
	sort.Slice(scanned, func(i, j int) bool { return scanned[i].Path < scanned[j].Path })
	return scanned
}

// outside: pnpm and npm print a JSON object whose entries each hold a "current"
// field; `go list -m -u -json all` prints one module object per entry, an
// "Update" object on the outdated ones. pnpm and npm exit 1 when something is outdated.
func (s *Service) outdated(ctx context.Context, dir, manager string) (count int, scanned bool) {
	var c system.Cmd
	var needle string
	switch manager {
	case "pnpm":
		c, needle = system.Cmd{Name: "pnpm", Args: []string{"outdated", "--dir", dir, "--format", "json"}}, `"current"`
	case "npm":
		c, needle = system.Cmd{Name: "npm", Args: []string{"outdated", "--json", "--prefix", dir}}, `"current"`
	case "go":
		c, needle = system.Cmd{Name: "go", Args: []string{"list", "-m", "-u", "-json", "all"}, Dir: dir}, `"Update"`
	default:
		return 0, true
	}
	r := s.ask(ctx, c, repoBudget)
	switch {
	case r.late:
		return -1, true
	case !r.found:
		return 0, false
	}
	return strings.Count(r.text(), needle), true
}

func (s *Service) docker(ctx context.Context) Docker {
	var (
		d                           Docker
		wg                          sync.WaitGroup
		containers, images, volumes reply
		dangling, total             reply
	)
	for _, job := range []struct {
		into *reply
		args []string
	}{
		{&containers, []string{"ps", "-q"}},
		{&images, []string{"images", "-q"}},
		{&volumes, []string{"volume", "ls", "-q"}},
		{&dangling, []string{"images", "-f", "dangling=true", "--format", "{{.Size}}"}},
		{&total, []string{"system", "df", "--format", "{{.Size}}"}},
	} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			*job.into = s.ask(ctx, system.Cmd{Name: "docker", Args: job.args}, defaultBudget)
		}()
	}
	wg.Wait()
	switch {
	case !containers.found:
		d.State = catalog.StateAbsent
	case containers.late:
		d.State = catalog.StateUnknown
	case !containers.ok():
		d.State = catalog.StateDegraded
	default:
		d.State = catalog.StateOK
		d.Containers = len(strings.Fields(containers.text()))
		d.Images = distinct(images.text())
		d.Volumes = len(strings.Fields(volumes.text()))
		d.DanglingBytes = sumSizes(dangling.text())
		d.TotalBytes = sumSizes(total.text())
	}
	return d
}

func distinct(text string) int {
	seen := map[string]bool{}
	for _, f := range strings.Fields(text) {
		seen[f] = true
	}
	return len(seen)
}

func sumSizes(text string) int64 {
	var sum int64
	for _, line := range strings.Split(text, "\n") {
		sum += dockerSize(strings.TrimSpace(line))
	}
	return sum
}

// outside: docker prints sizes in decimal units, `1.2GB`, `512MB`, `3.5kB`, `0B`.
func dockerSize(s string) int64 {
	for _, u := range []struct {
		suffix string
		factor float64
	}{{"TB", 1e12}, {"GB", 1e9}, {"MB", 1e6}, {"kB", 1e3}, {"B", 1}} {
		if num, ok := strings.CutSuffix(s, u.suffix); ok {
			n, err := strconv.ParseFloat(num, 64)
			if err != nil {
				return 0
			}
			return int64(n * u.factor)
		}
	}
	return 0
}

func (s *Service) disk() []Disk {
	home := s.h.Env.Home()
	sizes := []Disk{}
	for _, group := range s.diskGroups() {
		var bytes int64
		for _, p := range group.paths {
			bytes += s.h.FS.DirSize(path.Join(home, p))
		}
		if bytes > 0 {
			sizes = append(sizes, Disk{Name: group.name, Bytes: bytes})
		}
	}
	return sizes
}

type diskGroup struct {
	name  string
	paths []string
}

func (s *Service) diskGroups() []diskGroup {
	packages := []string{".npm", ".bun/install/cache", "go/pkg/mod", ".gradle/caches"}
	if !s.mac() {
		return []diskGroup{
			{"caches", []string{".cache"}},
			{"packages", packages},
			{"trash", []string{".local/share/Trash"}},
		}
	}
	return []diskGroup{
		{"homebrew", []string{"Library/Caches/Homebrew"}},
		{"xcode", []string{"Library/Developer/Xcode/DerivedData", "Library/Developer/Xcode/Archives", "Library/Developer/Xcode/iOS DeviceSupport"}},
		{"packages", append([]string{"Library/pnpm", "Library/Caches/Yarn", "Library/Caches/CocoaPods"}, packages...)},
		{"logs", []string{"Library/Logs"}},
		{"trash", []string{".Trash"}},
	}
}
