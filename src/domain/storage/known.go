package storage

import (
	"encoding/json"
	"path"
	"time"
)

const knownVersion = 1

type knownFile struct {
	V     int                   `json:"v"`
	Sizes map[string]knownEntry `json:"sizes"`
}

type knownEntry struct {
	Bytes int64     `json:"bytes"`
	At    time.Time `json:"at"`
}

func (s *Service) knownPath() string {
	if s.o.Expand.Roots.Cache == "" {
		return ""
	}
	return s.o.Expand.Expand("{cache}/station/sizes.json")
}

func (s *Service) loadKnown() map[string]knownEntry {
	file := s.knownPath()
	if file == "" {
		return nil
	}
	raw, err := s.h.FS.ReadFile(file)
	if err != nil {
		return nil
	}
	var k knownFile
	if json.Unmarshal(raw, &k) != nil || k.V != knownVersion {
		return nil
	}
	return k.Sizes
}

// invariant: last-known sizes are stale-while-revalidate, never a reason to skip
// a walk (a directory's mtime does not move when a file deeper down grows): every
// walk rewrites the file with what it answered, by an atomic rename.
func (s *Service) saveKnown(roots []string, sizes []int64) {
	file := s.knownPath()
	if file == "" {
		return
	}
	known := s.loadKnown()
	if known == nil {
		known = map[string]knownEntry{}
	}
	at := s.h.Clock.Now().UTC().Truncate(time.Second)
	for i, root := range roots {
		if sizes[i] >= 0 {
			known[root] = knownEntry{Bytes: sizes[i], At: at}
		}
	}
	raw, err := json.Marshal(knownFile{V: knownVersion, Sizes: known})
	if err != nil || s.h.FS.MkdirAll(path.Dir(file), 0o755) != nil {
		return
	}
	tmp := file + ".tmp"
	if s.h.FS.WriteFile(tmp, raw, 0o644) != nil {
		return
	}
	_ = s.h.FS.Rename(tmp, file)
}
