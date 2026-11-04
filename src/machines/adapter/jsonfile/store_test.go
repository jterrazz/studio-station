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
	if err != nil || len(r.Shares) != 1 || r.Shares[0] != "session:a" {
		t.Fatalf("Get = %+v, %v; want session:a lent", r, err)
	}
	r.Shares, r.Want = nil, "stopped"
	if err := s.Put(context.Background(), r); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(at); !strings.Contains(string(data), `"until": "2026-11-01T17:00:00Z"`) {
		t.Errorf("the share was lost:\n%s", data)
	}
}

func TestAReadableListingSkipsADamagedRecordButNotANewerOne(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "api.json"), []byte(`{"name":"api","image":"linux","version":"2026.11.01"}`), 0o600)
	_ = os.WriteFile(filepath.Join(dir, "x.json"), []byte(`{"name": "x", "ima`), 0o600)
	s := jsonfile.New(disk(), dir)
	if _, err := s.List(context.Background()); err == nil {
		t.Errorf("List passed a damaged record; want it named")
	}
	list, err := s.ListReadable(context.Background())
	if err != nil || len(list) != 1 || list[0].Name != "api" {
		t.Fatalf("ListReadable = %+v, %v; want api alone", list, err)
	}
	_ = os.WriteFile(filepath.Join(dir, "new.json"), []byte(`{"v":2,"name":"new","image":"linux"}`), 0o600)
	if _, err := s.ListReadable(context.Background()); err == nil {
		t.Errorf("ListReadable passed a record a newer station wrote; want it refused")
	}
}
