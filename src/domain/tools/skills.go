package tools

import (
	"context"
	"encoding/json"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/jterrazz/studio-station/src/catalog"
)

const currencyBudget = 5 * time.Second

// outside: mirrors ~/.agents/.skill-lock.json (schema version 3), the skills CLI's
// own record; only the fields the currency check reads are declared.
type lockEntry struct {
	Source     string `json:"source"`
	SourceType string `json:"sourceType"`
	SkillPath  string `json:"skillPath"`
	FolderHash string `json:"skillFolderHash"`
}

func (s *Service) readLock() map[string]lockEntry {
	var doc struct {
		Skills map[string]lockEntry `json:"skills"`
	}
	data, err := s.h.FS.ReadFile(path.Join(s.h.Env.Home(), ".agents", ".skill-lock.json"))
	if err != nil || json.Unmarshal(data, &doc) != nil {
		return map[string]lockEntry{}
	}
	return doc.Skills
}

// invariant: the verdict compares git tree SHAs, the oracle the skills CLI updates
// by; an inconclusive check (network, non-200, a folder gone upstream) leaves the
// skill ok with its currency unknown, never outdated.
func (s *Service) currency(ctx context.Context, reports []Report, l listing) {
	ctx, cancel := context.WithTimeout(ctx, currencyBudget)
	defer cancel()
	var mu sync.Mutex
	folders := map[string]map[string]string{}
	listingOf := func(source, parent string) map[string]string {
		mu.Lock()
		defer mu.Unlock()
		key := source + "\x00" + parent
		if shas, ok := folders[key]; ok {
			return shas
		}
		shas := s.fetchFolder(ctx, source, parent)
		folders[key] = shas
		return shas
	}
	var wg sync.WaitGroup
	for i := range reports {
		r := &reports[i]
		if r.Method != catalog.MethodSkill || r.State != catalog.StateOK {
			continue
		}
		entry := l.skills[s.install(r.Tool).Key()]
		wg.Go(func() {
			folder := path.Dir(entry.SkillPath)
			if entry.SourceType != "github" || entry.FolderHash == "" || folder == "." {
				r.Detail = "currency unknown"
				return
			}
			parent := path.Dir(folder)
			if parent == "." {
				parent = ""
			}
			upstream, ok := listingOf(entry.Source, parent)[path.Base(folder)]
			switch {
			case !ok:
				r.Detail = "currency unknown"
			case upstream != entry.FolderHash:
				r.State, r.Detail, r.Why = catalog.StateDegraded, "outdated", WhyOutdated
			}
		})
	}
	wg.Wait()
}

func (s *Service) fetchFolder(ctx context.Context, source, parent string) map[string]string {
	header := map[string]string{"Accept": "application/vnd.github+json"}
	// outside: unauthenticated api.github.com allows 60 requests an hour, a token
	// 5000; a rate-limited answer is a non-200, so the currency stays unknown.
	if s.o.GitHubToken != "" {
		header["Authorization"] = "Bearer " + s.o.GitHubToken
	}
	url := strings.TrimSuffix(s.o.APIURL, "/") + "/repos/" + source + "/contents/" + parent
	status, body, err := s.h.Net.Get(ctx, url, header)
	if err != nil || status != 200 {
		return nil
	}
	var entries []struct {
		Name string `json:"name"`
		SHA  string `json:"sha"`
	}
	if json.Unmarshal(body, &entries) != nil {
		return nil
	}
	shas := make(map[string]string, len(entries))
	for _, e := range entries {
		shas[e.Name] = e.SHA
	}
	return shas
}
