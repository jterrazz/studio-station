package config

import (
	"testing"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/platform/local/localtest"
)

func TestAnEffectNobodyHandlesIsAnErrorNotASilentSuccess(t *testing.T) {
	h := localtest.Host(localtest.NewRunner(t), localtest.NewFS(), localtest.Env{HomeDir: "/home/u", OS: "darwin"}, localtest.NewNet(), &localtest.Clock{})
	svc := New(h, catalog.ConfigItems, Options{Here: catalog.Host})
	if _, err := svc.handler(catalog.ConfigItem{Name: "x", Effect: catalog.Procedure("nope")}); err == nil {
		t.Fatal("an unknown procedure dispatched")
	}
	if _, err := svc.handler(catalog.ConfigItem{Name: "y"}); err == nil {
		t.Fatal("an item with no effect dispatched")
	}
}
