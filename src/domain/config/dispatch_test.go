package config

import (
	"testing"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/system/systemtest"
)

func TestAnEffectNobodyHandlesIsAnErrorNotASilentSuccess(t *testing.T) {
	h := systemtest.Host(systemtest.NewRunner(t), systemtest.NewFS(), systemtest.Env{HomeDir: "/home/u", OS: "darwin"}, systemtest.NewNet(), &systemtest.Clock{})
	svc := New(h, catalog.Settings, Options{Here: catalog.Mac})
	if _, err := svc.handler(catalog.Setting{Name: "x", Effect: catalog.Procedure("nope")}); err == nil {
		t.Fatal("an unknown procedure dispatched")
	}
	if _, err := svc.handler(catalog.Setting{Name: "y"}); err == nil {
		t.Fatal("an item with no effect dispatched")
	}
}
