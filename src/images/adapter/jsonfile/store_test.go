package jsonfile_test

import (
	"testing"

	"github.com/jterrazz/studio-station/src/images/adapter/jsonfile"
	"github.com/jterrazz/studio-station/src/images/contract"
	"github.com/jterrazz/studio-station/src/images/contract/contracttest"
	"github.com/jterrazz/studio-station/src/platform/local"
)

func TestImageStore(t *testing.T) {
	contracttest.ImageStore(t, func(t *testing.T) contract.ImageStore {
		return jsonfile.New(local.New(nil, nil, nil, nil).FS, t.TempDir())
	})
}
