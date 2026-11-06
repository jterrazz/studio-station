package calls

import (
	"testing"

	"github.com/jterrazz/studio-station/src/catalog"
)

func TestAnEffectNobodyHandlesIsAnErrorNotASilentSuccess(t *testing.T) {
	svc := NewCatalogue(catalog.ConfigItems, CatalogueOptions{Here: catalog.Host})
	if _, err := svc.handler(catalog.ConfigItem{Name: "x", Effect: catalog.Procedure("nope")}); err == nil {
		t.Fatal("an unknown procedure dispatched")
	}
	if _, err := svc.handler(catalog.ConfigItem{Name: "y"}); err == nil {
		t.Fatal("an item with no effect dispatched")
	}
}
