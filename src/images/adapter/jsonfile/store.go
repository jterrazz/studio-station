package jsonfile

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"slices"
	"strings"

	"github.com/jterrazz/studio-station/src/core/store"
	"github.com/jterrazz/studio-station/src/images"
	"github.com/jterrazz/studio-station/src/images/contract"
)

const Version = 1

type Files interface {
	ReadFile(path string) ([]byte, error)
	WriteFile(path string, data []byte, perm fs.FileMode) error
	MkdirAll(path string, perm fs.FileMode) error
	ReadDir(path string) ([]fs.DirEntry, error)
	Remove(path string) error
}

type Store struct {
	files Files
	dir   string
}

var (
	_ contract.ImageStore = (*Store)(nil)
	_ store.Migrator      = (*Store)(nil)
)

func New(files Files, dir string) *Store { return &Store{files: files, dir: dir} }

type file struct {
	images.Record
	V int `json:"v,omitempty"`
}

func (s *Store) imageDir(image string) string { return path.Join(s.dir, image) }

func (s *Store) file(image, version string) string {
	return path.Join(s.imageDir(image), version+".json")
}

func (s *Store) read(at string) (images.Record, error) {
	data, err := s.files.ReadFile(at)
	if err != nil {
		return images.Record{}, err
	}
	var f file
	if err := json.Unmarshal(data, &f); err != nil {
		return images.Record{}, fmt.Errorf("image record %s: %w", at, err)
	}
	if _, err := store.Current(f.V, Version); err != nil {
		return images.Record{}, fmt.Errorf("image record %s: %w", at, err)
	}
	return f.Record, nil
}

func (s *Store) Get(_ context.Context, image, version string) (images.Record, error) {
	r, err := s.read(s.file(image, version))
	if errors.Is(err, fs.ErrNotExist) {
		return images.Record{}, store.ErrNotFound
	}
	return r, err
}

func (s *Store) Put(_ context.Context, r images.Record) error {
	if err := s.files.MkdirAll(s.imageDir(r.Name), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(file{Record: r}, "", "  ")
	if err != nil {
		return err
	}
	return s.files.WriteFile(s.file(r.Name, r.Version), append(data, '\n'), 0o600)
}

func (s *Store) List(_ context.Context, image string) ([]images.Record, error) {
	entries, err := s.files.ReadDir(s.imageDir(image))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []images.Record
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		r, err := s.read(path.Join(s.imageDir(image), e.Name()))
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	slices.SortFunc(out, func(a, b images.Record) int { return images.CompareVersions(b.Version, a.Version) })
	return out, nil
}

func (s *Store) Delete(_ context.Context, image, version string) error {
	if err := s.files.Remove(s.file(image, version)); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

func (s *Store) Migrate(ctx context.Context) error {
	entries, err := s.files.ReadDir(s.dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() {
			if _, err := s.List(ctx, e.Name()); err != nil {
				return err
			}
		}
	}
	return nil
}
