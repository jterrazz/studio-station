package fake

import (
	"context"
	"slices"

	"github.com/jterrazz/studio-station/src/core/store"
	"github.com/jterrazz/studio-station/src/images"
	"github.com/jterrazz/studio-station/src/images/contract"
)

type Store struct{ records map[[2]string]images.Record }

var (
	_ contract.ImageStore = (*Store)(nil)
	_ store.Migrator      = (*Store)(nil)
)

func NewStore() *Store { return &Store{records: map[[2]string]images.Record{}} }

func (s *Store) Get(_ context.Context, image, version string) (images.Record, error) {
	r, ok := s.records[[2]string{image, version}]
	if !ok {
		return images.Record{}, store.ErrNotFound
	}
	return r, nil
}

func (s *Store) Put(_ context.Context, r images.Record) error {
	s.records[[2]string{r.Name, r.Version}] = r
	return nil
}

func (s *Store) List(_ context.Context, image string) ([]images.Record, error) {
	var out []images.Record
	for key, r := range s.records {
		if key[0] == image {
			out = append(out, r)
		}
	}
	slices.SortFunc(out, func(a, b images.Record) int { return images.CompareVersions(b.Version, a.Version) })
	return out, nil
}

func (s *Store) Delete(_ context.Context, image, version string) error {
	delete(s.records, [2]string{image, version})
	return nil
}

func (s *Store) Migrate(context.Context) error { return nil }
