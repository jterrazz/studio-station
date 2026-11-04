package config

import (
	"errors"
	"testing"
	"testing/fstest"

	accountscontract "github.com/jterrazz/studio-station/src/accounts/contract"
	corefake "github.com/jterrazz/studio-station/src/core/adapter/fake"
	setupcontract "github.com/jterrazz/studio-station/src/setup/contract"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/domain/settings"
	"github.com/jterrazz/studio-station/src/system/systemtest"
)

func newTestService(t *testing.T, here catalog.MachineKind, euid int) (*Service, *systemtest.Runner, *systemtest.FS) {
	t.Helper()
	run, files := systemtest.NewRunner(t), systemtest.NewFS()
	env := systemtest.Env{HomeDir: "/home/u", UID: euid, OS: "darwin"}
	h := systemtest.Host(run, files, env, systemtest.NewNet(), &systemtest.Clock{})
	paths := settings.Paths{Config: "/home/u/.config/station", State: "/home/u/.local/state/station"}
	svc := New(h, catalog.ConfigItems, fstest.MapFS{}, Prefs{Identity: setupcontract.Identity(corefake.NewPreferences()), ShareList: accountscontract.ShareList(corefake.NewPreferences())}, Options{Here: here, DNSProfile: paths.State + "/dns/quad9-dns.mobileconfig", BackupExclusions: []string{"/home/u/.cache", "/home/u/.local/share/station/vms"}})
	return svc, run, files
}

func TestEveryCatalogueItemDispatchesToAHandlerForEveryOperationItDeclares(t *testing.T) {
	svc, _, _ := newTestService(t, catalog.Host, 0)
	for _, item := range catalog.ConfigItems {
		h, err := svc.handler(item)
		var unhandled *unhandledError
		if errors.As(err, &unhandled) || err != nil {
			t.Errorf("%s: no handler for %T %v", item.Name, item.Effect, item.Effect)
			continue
		}
		if h.install == nil {
			t.Errorf("%s: no install", item.Name)
		}
		if h.check == nil && !item.RunOnce {
			t.Errorf("%s: no check, and it is not run once", item.Name)
		}
		if (h.uninstall != nil) != item.Reversible {
			t.Errorf("%s: reversible=%v but uninstall handled=%v", item.Name, item.Reversible, h.uninstall != nil)
		}
	}
}

func TestAnEffectNobodyHandlesIsAnErrorNotASilentSuccess(t *testing.T) {
	svc, _, _ := newTestService(t, catalog.Host, 0)
	if _, err := svc.handler(catalog.ConfigItem{Name: "x", Effect: catalog.Procedure("nope")}); err == nil {
		t.Fatal("an unknown procedure dispatched")
	}
	if _, err := svc.handler(catalog.ConfigItem{Name: "y"}); err == nil {
		t.Fatal("an item with no effect dispatched")
	}
}
