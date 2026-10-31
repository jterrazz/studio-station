package catalog_test

import (
	"slices"
	"testing"

	"github.com/jterrazz/studio-station/src/catalog"
)

func TestPlaceEveryInstallOnAKindItsMethodAccepts(t *testing.T) {
	for _, tool := range catalog.Tools {
		seen := map[catalog.MachineKind]bool{}
		for _, place := range tool.Installs {
			accepts := catalog.MethodKinds[place.Install.Method()]
			for _, kind := range place.Kinds {
				if seen[kind] {
					t.Errorf("%s: kind %s placed twice", tool.Name, kind)
				}
				seen[kind] = true
				if !slices.Contains(accepts, kind) {
					t.Errorf("%s: method %s does not run on %s", tool.Name, place.Install.Method(), kind)
				}
			}
		}
		if len(seen) == 0 {
			t.Errorf("%s: placed nowhere", tool.Name)
		}
	}
}

func TestPlaceBundleMethodsNeverReachLinux(t *testing.T) {
	for _, m := range []catalog.Method{catalog.MethodBrew, catalog.MethodCask, catalog.MethodMas, catalog.MethodApp, catalog.MethodNpm, catalog.MethodUv} {
		if slices.Contains(catalog.MethodKinds[m], catalog.Linux) {
			t.Errorf("method %s accepts linux", m)
		}
	}
}

func TestUniqueNamesAcrossToolsSkillsAndConfigItems(t *testing.T) {
	owner := map[string]string{}
	claim := func(name, what string) {
		if prev, ok := owner[name]; ok {
			t.Errorf("%q names both a %s and a %s", name, prev, what)
		}
		owner[name] = what
	}
	for _, tool := range catalog.Tools {
		claim(tool.Name, "tool")
	}
	for _, item := range catalog.ConfigItems {
		claim(item.Name, "config item")
	}
}

func TestEveryRequirementAndThenNamesACatalogueEntry(t *testing.T) {
	tools := map[string]bool{}
	for _, tool := range catalog.Tools {
		tools[tool.Name] = true
	}
	items := map[string]bool{}
	for _, item := range catalog.ConfigItems {
		items[item.Name] = true
	}
	for _, tool := range catalog.Tools {
		for _, r := range tool.Requires {
			if !tools[r] {
				t.Errorf("%s requires the unknown tool %q", tool.Name, r)
			}
		}
		for _, then := range tool.Then {
			if !items[then] {
				t.Errorf("%s runs the unknown config item %q", tool.Name, then)
			}
		}
	}
}
