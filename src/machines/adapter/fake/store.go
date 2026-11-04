package fake

import (
	"context"
	"maps"
	"slices"

	"github.com/jterrazz/studio-station/src/core/store"
	"github.com/jterrazz/studio-station/src/machines"
	"github.com/jterrazz/studio-station/src/machines/contract"
)

type Store struct{ records map[string]machines.Record }

var (
	_ contract.MachineStore = (*Store)(nil)
	_ store.Migrator        = (*Store)(nil)
)

func NewStore() *Store { return &Store{records: map[string]machines.Record{}} }

func (s *Store) Get(_ context.Context, name string) (machines.Record, error) {
	r, ok := s.records[name]
	if !ok {
		return machines.Record{}, store.ErrNotFound
	}
	return r, nil
}

func (s *Store) Put(_ context.Context, r machines.Record) error {
	r.Shares = s.records[r.Name].Shares
	s.records[r.Name] = r
	return nil
}

func (s *Store) List(context.Context) ([]machines.Record, error) {
	out := make([]machines.Record, 0, len(s.records))
	for _, name := range slices.Sorted(maps.Keys(s.records)) {
		out = append(out, s.records[name])
	}
	return out, nil
}

func (s *Store) ListReadable(ctx context.Context) ([]machines.Record, error) { return s.List(ctx) }

func (s *Store) Delete(_ context.Context, name string) error {
	delete(s.records, name)
	return nil
}

func (s *Store) Migrate(context.Context) error { return nil }
