package tools

import (
	"testing"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/system"
)

func TestMethodEveryCatalogueMethodHasOneRowAndOneInstaller(t *testing.T) {
	table := methods()
	for m := range catalog.MethodKinds {
		row, ok := table[m]
		if !ok {
			t.Errorf("%s: placed by the catalogue, dispatched by no row", m)
			continue
		}
		if row.bundleLine && row.install != nil {
			t.Errorf("%s: installed both by brew bundle and by hand", m)
		}
		if !row.bundleLine && row.install == nil && row.uninstall != nil {
			t.Errorf("%s: uninstalls what nothing installs", m)
		}
	}
	if len(table) != len(catalog.MethodKinds) {
		t.Errorf("%d rows for the %d methods the catalogue places", len(table), len(catalog.MethodKinds))
	}
}

func TestMethodEveryInstallVariantIsDispatched(t *testing.T) {
	variants := []catalog.Install{
		catalog.TrustedBrew("hashicorp/tap/terraform"), catalog.Cask("ghostty"), catalog.Mas("Xcode", 497799835),
		catalog.Npm("@playwright/mcp"), catalog.Uv("markitdown[all]"), catalog.BunAs("github:tobi/qmd", "qmd"),
		catalog.Mise("node", "lts"), catalog.ScriptEnv("https://x/install.sh", []string{"A=1"}, "--flag"),
		catalog.Shell("true"), catalog.App("by hand"), catalog.Skill("anthropics/skills", "frontend-design"),
	}
	table := methods()
	for _, v := range variants {
		row, ok := table[v.Method()]
		if !ok {
			t.Errorf("%T: method %s has no row", v, v.Method())
			continue
		}
		if row.bundleLine && brewfileLine(v) == "" {
			t.Errorf("%T: a bundle method without a Brewfile line", v)
		}
		for name, build := range map[string]func(catalog.Install) []system.Cmd{
			"install":   row.install,
			"uninstall": row.uninstall,
			"upgrade":   row.upgrade,
		} {
			if build != nil && len(build(v)) == 0 {
				t.Errorf("%T: the %s column builds no command", v, name)
			}
		}
	}
}
