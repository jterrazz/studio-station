package fake

import (
	"testing"

	"github.com/jterrazz/studio-station/src/images/contract"
	"github.com/jterrazz/studio-station/src/images/contract/contracttest"
)

func TestTheFakeImageStoreKeepsTheContract(t *testing.T) {
	contracttest.ImageStore(t, func(*testing.T) contract.ImageStore { return NewStore() })
}
