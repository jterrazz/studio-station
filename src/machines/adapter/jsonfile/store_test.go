package jsonfile_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jterrazz/studio-station/src/machines/adapter/jsonfile"
	"github.com/jterrazz/studio-station/src/machines/contract"
	"github.com/jterrazz/studio-station/src/machines/contract/contracttest"
	"github.com/jterrazz/studio-station/src/platform/local"
)

func disk() jsonfile.Files { return local.New(nil, nil, nil, nil).FS }

func TestMachineStore(t *testing.T) {
	contracttest.MachineStore(t, func(t *testing.T) contract.MachineStore {
		return jsonfile.New(disk(), filepath.Join(t.TempDir(), "vms"))
	})
}

func TestAPutKeepsTheSharesBesideTheRecord(t *testing.T) {
	dir := t.TempDir()
	at := filepath.Join(dir, "api.json")
	_ = os.WriteFile(at, []byte(`{"name":"api","image":"linux","lent":[{"name":"session:a","at":"2026-11-01T09:00:00Z","until":"2026-11-01T17:00:00Z"}]}`), 0o600)
	s := jsonfile.New(disk(), dir)
	r, err := s.Get(context.Background(), "api")
	if err != nil || len(r.Lent) != 1 || r.Lent[0] != "session:a" {
		t.Fatalf("Get = %+v, %v; want session:a lent", r, err)
	}
	r.Lent, r.Want = nil, "stopped"
	if err := s.Put(context.Background(), r); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(at); !strings.Contains(string(data), `"until": "2026-11-01T17:00:00Z"`) {
		t.Errorf("the share was lost:\n%s", data)
	}
}
