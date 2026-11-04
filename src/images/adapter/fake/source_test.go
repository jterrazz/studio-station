package fake

import (
	"testing"

	"github.com/jterrazz/studio-station/src/images"
	"github.com/jterrazz/studio-station/src/images/contract"
	"github.com/jterrazz/studio-station/src/images/contract/contracttest"
	"github.com/jterrazz/studio-station/src/world"
)

func TestTheFakeRegistryKeepsTheContract(t *testing.T) {
	contracttest.ImageSource(t, func(_ *testing.T, versions ...string) contract.ImageSource {
		w := &world.World{Registry: map[string]string{}}
		for _, v := range versions {
			if images.CompareVersions(v, w.Registry["linux"]) > 0 {
				w.Registry["linux"] = v
			}
		}
		return SourceOf(w, contracttest.Hypervisor)
	})
}
