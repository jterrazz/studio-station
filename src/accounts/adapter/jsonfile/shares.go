package jsonfile

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"path"
	"strings"
	"time"

	"github.com/jterrazz/studio-station/src/accounts"
	"github.com/jterrazz/studio-station/src/accounts/contract"
	"github.com/jterrazz/studio-station/src/core/store"
)

type Files interface {
	ReadFile(path string) ([]byte, error)
	WriteFile(path string, data []byte, perm fs.FileMode) error
	ReadDir(path string) ([]fs.DirEntry, error)
	Stat(path string) (fs.FileInfo, error)
}

type Shares struct {
	// invariant: the shares live beside each VM's record, under its `lent`
	// key; only that key is read or written, every other key is carried over
	// untouched, since the machines' store owns the rest of the file.
	files Files
	dir   string
}

var (
	_ contract.ShareStore = (*Shares)(nil)
	_ store.Migrator      = (*Shares)(nil)
)

func NewShares(files Files, dir string) *Shares { return &Shares{files: files, dir: dir} }

func (s *Shares) file(vm string) string { return path.Join(s.dir, vm+".json") }

func (s *Shares) Get(_ context.Context, vm string) ([]accounts.Share, error) {
	_, shares, err := s.read(vm)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	return shares, err
}

func (s *Shares) Put(_ context.Context, vm string, shares []accounts.Share) error {
	record, _, err := s.read(vm)
	if err != nil {
		return err
	}
	if len(shares) == 0 {
		delete(record, "lent")
	} else {
		raw, marshalErr := json.Marshal(shares)
		if marshalErr != nil {
			return marshalErr
		}
		record["lent"] = raw
	}
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return err
	}
	return s.files.WriteFile(s.file(vm), append(data, '\n'), 0o600)
}

func (s *Shares) List(context.Context) (map[string][]accounts.Share, error) {
	held := map[string][]accounts.Share{}
	entries, err := s.files.ReadDir(s.dir)
	if errors.Is(err, fs.ErrNotExist) {
		return held, nil
	}
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		vm, ok := strings.CutSuffix(e.Name(), ".json")
		if !ok {
			continue
		}
		_, shares, err := s.read(vm)
		if err != nil {
			return nil, err
		}
		if len(shares) > 0 {
			held[vm] = shares
		}
	}
	return held, nil
}

func (s *Shares) Delete(ctx context.Context, vm string) error {
	shares, err := s.Get(ctx, vm)
	if err != nil || len(shares) == 0 {
		return err
	}
	return s.Put(ctx, vm, nil)
}

func (s *Shares) Migrate(ctx context.Context) error {
	_, err := s.List(ctx)
	return err
}

// invariant: a record written before shares expired holds plain names: each
// reads as shared at the file's mtime for the default term, and the next put
// writes it in the new form.
func (s *Shares) read(vm string) (record map[string]json.RawMessage, shares []accounts.Share, err error) {
	data, err := s.files.ReadFile(s.file(vm))
	if err != nil {
		return nil, nil, err
	}
	record = map[string]json.RawMessage{}
	if err := json.Unmarshal(data, &record); err != nil {
		return nil, nil, err
	}
	raw, ok := record["lent"]
	if !ok {
		return record, nil, nil
	}
	var objects []accounts.Share
	if json.Unmarshal(raw, &objects) == nil {
		return record, objects, nil
	}
	var names []string
	if err := json.Unmarshal(raw, &names); err != nil {
		return nil, nil, err
	}
	at := time.Time{}
	if info, statErr := s.files.Stat(s.file(vm)); statErr == nil {
		at = info.ModTime().UTC().Truncate(time.Second)
	}
	for _, n := range names {
		shares = append(shares, accounts.Share{Name: n, At: at, Until: at.Add(accounts.DefaultShareTerm)})
	}
	return record, shares, nil
}
