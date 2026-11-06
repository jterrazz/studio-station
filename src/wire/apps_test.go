package wire

import (
	"testing"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/platform/local/localtest"
)

func TestEveryMethodTheCataloguePlacesHasItsInstaller(t *testing.T) {
	system := localtest.Host(localtest.NewRunner(t), localtest.NewFS(), localtest.Env{HomeDir: "/home/u", OS: "darwin"}, localtest.NewNet(), &localtest.Clock{})

	registry, _ := installers(machine{sys: system, vars: map[string]string{}})

	for m := range catalog.MethodKinds {
		if i, ok := registry[m]; !ok || i.Method() != m {
			t.Errorf("%s: placed by the catalogue, installed by no adapter", m)
		}
	}
	if len(registry) != len(catalog.MethodKinds) {
		t.Errorf("%d installers for the %d methods the catalogue places", len(registry), len(catalog.MethodKinds))
	}
}
