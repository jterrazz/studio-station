package jsonfile_test

import (
	"os"
	"path/filepath"
	"testing"

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
