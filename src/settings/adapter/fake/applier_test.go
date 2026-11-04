package fake_test

import (
	"testing"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/settings/adapter/fake"
	"github.com/jterrazz/studio-station/src/settings/contract/contracttest"
)

func TestApplier(t *testing.T) {
	contracttest.Applier(t, func(*testing.T) contracttest.Opened {
		a := fake.NewApplier(catalog.EffectDotfile)
		return contracttest.Opened{Applier: a, Item: catalog.ConfigItem{Name: "starship"}, Edit: func(*testing.T) { a.Edit("starship", "yours") }}
	})
}
