package app

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"path"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jterrazz/studio-station/src/cli/kit"
	secretscmd "github.com/jterrazz/studio-station/src/cli/secrets"
	secretsallow "github.com/jterrazz/studio-station/src/cli/secrets/allow"
	secretsdisallow "github.com/jterrazz/studio-station/src/cli/secrets/disallow"
	secretslend "github.com/jterrazz/studio-station/src/cli/secrets/lend"
	secretsrecall "github.com/jterrazz/studio-station/src/cli/secrets/recall"
	"github.com/jterrazz/studio-station/src/system"
)

func secretsCommand(env kit.Env, m machine) *cobra.Command {
	service := m.secrets()
	return secretscmd.Command(secretscmd.Deps{Secrets: service, Here: m.where}, env,
		secretsallow.Command(secretsallow.Deps{Secrets: service, Here: m.where}, env),
		secretsdisallow.Command(secretsdisallow.Deps{Secrets: service, Here: m.where}, env),
		secretslend.Command(secretslend.Deps{Secrets: service, Here: m.where}, env),
		secretsrecall.Command(secretsrecall.Deps{Secrets: service, Here: m.where}, env),
	)
}

// invariant: only the `lent` key of a record is read or written; every other key
// is carried over untouched, since domain/machines owns the rest of the file.
type vmRecords struct {
	fs  system.FS
	dir string
}

func (r vmRecords) IsVM(_ context.Context, name string) (bool, error) {
	_, err := r.fs.Stat(r.file(name))
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	return err == nil, err
}

func (r vmRecords) RecordLent(name string, names []string) error {
	return r.update(name, func(lent []string) []string {
		for _, n := range names {
			if !slices.Contains(lent, n) {
				lent = append(lent, n)
			}
		}
		return lent
	})
}

func (r vmRecords) RecordRecalled(name string, names []string) error {
	return r.update(name, func(lent []string) []string {
		return slices.DeleteFunc(lent, func(n string) bool { return slices.Contains(names, n) })
	})
}

func (r vmRecords) Holding(names []string) (map[string][]string, error) {
	held := map[string][]string{}
	entries, err := r.fs.ReadDir(r.dir)
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
		_, lent, err := r.read(vm)
		if err != nil {
			return nil, err
		}
		if names != nil {
			lent = slices.DeleteFunc(lent, func(n string) bool { return !slices.Contains(names, n) })
		}
		if len(lent) > 0 {
			held[vm] = lent
		}
	}
	return held, nil
}

func (r vmRecords) update(name string, change func([]string) []string) error {
	record, lent, err := r.read(name)
	if err != nil {
		return err
	}
	lent = change(lent)
	if len(lent) == 0 {
		delete(record, "lent")
	} else {
		raw, marshalErr := json.Marshal(lent)
		if marshalErr != nil {
			return marshalErr
		}
		record["lent"] = raw
	}
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return err
	}
	return r.fs.WriteFile(r.file(name), append(data, '\n'), 0o600)
}

func (r vmRecords) read(name string) (record map[string]json.RawMessage, lent []string, err error) {
	data, err := r.fs.ReadFile(r.file(name))
	if err != nil {
		return nil, nil, err
	}
	record = map[string]json.RawMessage{}
	if err := json.Unmarshal(data, &record); err != nil {
		return nil, nil, err
	}
	if raw, ok := record["lent"]; ok {
		if err := json.Unmarshal(raw, &lent); err != nil {
			return nil, nil, err
		}
	}
	return record, lent, nil
}

func (r vmRecords) file(name string) string { return path.Join(r.dir, name+".json") }
