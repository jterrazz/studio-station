package wire

import (
	"testing"

	"github.com/jterrazz/studio-station/src/catalog"
	settingscontract "github.com/jterrazz/studio-station/src/settings/contract"
	"github.com/jterrazz/studio-station/src/system/systemtest"
)

func TestEverySettingOfTheCatalogueIsWrittenByAnApplierOnARealMac(t *testing.T) {
	host := systemtest.Host(systemtest.NewRunner(t), systemtest.NewFS(), systemtest.Env{HomeDir: "/home/u", OS: "darwin"}, systemtest.NewNet(), &systemtest.Clock{})

	appliers := settingscontract.NewAppliers(settingAppliers(machine{sys: host, vars: map[string]string{}, stores: &stores{}})...)

	for _, it := range catalog.Settings {
		if _, ok := appliers.For(it); !ok {
			t.Errorf("%s: its effect %v is written by no Applier", it.Name, it.Effect)
		}
	}
}
