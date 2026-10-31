package images

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"slices"
	"strconv"
	"strings"
	"time"
)

type Record struct {
	Name      string    `json:"name"`
	Version   string    `json:"version"`
	Base      string    `json:"base"`
	Station   string    `json:"station"`
	RecipeSHA string    `json:"recipe_sha"`
	BuiltAt   time.Time `json:"built_at"`
	SizeGB    int       `json:"size_gb"`
}

const (
	imagePrefix = "station-image-"
	buildPrefix = "station-build-"
)

func TartName(image, version string) string { return imagePrefix + image + "-" + version }

func (s *Service) imageDir(image string) string { return path.Join(s.o.Paths.State, "images", image) }

func (s *Service) records(image string) ([]Record, error) {
	dir := s.imageDir(image)
	entries, err := s.h.FS.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []Record
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := s.h.FS.ReadFile(path.Join(dir, e.Name()))
		if err != nil {
			return nil, err
		}
		var r Record
		if err := json.Unmarshal(data, &r); err != nil {
			return nil, fmt.Errorf("image record %s: %w", path.Join(dir, e.Name()), err)
		}
		out = append(out, r)
	}
	slices.SortFunc(out, func(a, b Record) int { return compareVersions(b.Version, a.Version) })
	return out, nil
}

func (s *Service) writeRecord(r Record) error {
	if err := s.h.FS.MkdirAll(s.imageDir(r.Name), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	return s.h.FS.WriteFile(path.Join(s.imageDir(r.Name), r.Version+".json"), append(data, '\n'), 0o600)
}

func compareVersions(a, b string) int {
	pa, pb := strings.Split(a, "."), strings.Split(b, ".")
	for i := range max(len(pa), len(pb)) {
		if c := part(pa, i) - part(pb, i); c != 0 {
			return c
		}
	}
	return 0
}

func part(parts []string, i int) int {
	if i >= len(parts) {
		return 1
	}
	n, err := strconv.Atoi(parts[i])
	if err != nil {
		return 0
	}
	return n
}

func (s *Service) nextVersion(image string) (string, error) {
	existing, err := s.records(image)
	if err != nil {
		return "", err
	}
	taken := make(map[string]bool, len(existing))
	for _, r := range existing {
		taken[r.Version] = true
	}
	day := s.h.Clock.Now().UTC().Format("2006.01.02")
	if !taken[day] {
		return day, nil
	}
	for n := 2; ; n++ {
		if candidate := day + "." + strconv.Itoa(n); !taken[candidate] {
			return candidate, nil
		}
	}
}
