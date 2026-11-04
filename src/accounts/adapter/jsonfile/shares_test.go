package jsonfile_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/jterrazz/studio-station/src/accounts"
	"github.com/jterrazz/studio-station/src/accounts/adapter/jsonfile"
	"github.com/jterrazz/studio-station/src/accounts/contract"
	"github.com/jterrazz/studio-station/src/accounts/contract/contracttest"
	"github.com/jterrazz/studio-station/src/platform/local"
)

func TestShareStore(t *testing.T) {
	contracttest.ShareStore(t, func(t *testing.T, vms ...string) contract.ShareStore {
		dir := t.TempDir()
		for _, vm := range vms {
			if err := os.WriteFile(filepath.Join(dir, vm+".json"), []byte(`{"name":"`+vm+`"}`), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		return jsonfile.NewShares(local.New(nil, nil, nil, nil).FS, dir)
	})
}

func TestASharePutKeepsTheRecordBesideIt(t *testing.T) {
	dir := t.TempDir()
	at := filepath.Join(dir, "api.json")
	record := `{"name":"api","image":"linux","cpus":4,"created_at":"2026-11-01T09:00:00Z"}`
	if err := os.WriteFile(at, []byte(record), 0o600); err != nil {
		t.Fatal(err)
	}
	s := jsonfile.NewShares(local.New(nil, nil, nil, nil).FS, dir)
	nine := time.Date(2026, 11, 1, 9, 0, 0, 0, time.UTC)
	if err := s.Put(context.Background(), "api", []accounts.Share{{Name: "session:a", At: nine, Until: nine.Add(accounts.DefaultShareTerm)}}); err != nil {
		t.Fatal(err)
	}
	if err := s.Put(context.Background(), "api", nil); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(at)
	var got, want map[string]any
	_ = json.Unmarshal(data, &got)
	_ = json.Unmarshal([]byte(record), &want)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("the VM's record changed:\n%s", data)
	}
}
