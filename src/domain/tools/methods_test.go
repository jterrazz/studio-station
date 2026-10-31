package tools

import (
	"slices"
	"testing"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/system"
)

type columns struct {
	kinds                               []catalog.MachineKind
	bundle, install, uninstall, upgrade bool
	inventory, needs                    string
}

func TestMethodTableHasSectionCsColumnsForEveryMethod(t *testing.T) {
	mac := []catalog.MachineKind{catalog.Host, catalog.MacOS}
	all := []catalog.MachineKind{catalog.Host, catalog.Linux, catalog.MacOS}
	const none = ""
	want := map[catalog.Method]columns{
		catalog.MethodBrew:   {mac, true, false, true, true, "brew list --formula --versions", "homebrew"},
		catalog.MethodCask:   {mac, true, false, true, true, "brew list --cask --versions", "homebrew"},
		catalog.MethodMas:    {mac, true, false, true, true, "mas list", "homebrew"},
		catalog.MethodNpm:    {mac, true, false, true, true, "npm ls -g --depth=0 --json", "homebrew"},
		catalog.MethodUv:     {mac, true, false, true, true, "uv tool list", "homebrew"},
		catalog.MethodBun:    {all, false, true, true, true, "bun pm ls -g", none},
		catalog.MethodMise:   {all, false, true, true, true, "mise ls --global --json", "mise"},
		catalog.MethodScript: {all, false, true, false, true, none, none},
		catalog.MethodShell:  {all, false, true, false, false, none, none},
		catalog.MethodApp:    {mac, false, false, false, false, none, none},
		catalog.MethodSkill:  {all, false, true, true, true, none, "skills"},
	}
	table := methods()
	for _, m := range []catalog.Method{catalog.MethodBrew, catalog.MethodCask, catalog.MethodMas, catalog.MethodNpm, catalog.MethodUv, catalog.MethodBun, catalog.MethodMise, catalog.MethodScript, catalog.MethodShell, catalog.MethodApp, catalog.MethodSkill} {
		row, ok := table[m]
		if !ok {
			t.Errorf("%s: no row", m)
			continue
		}
		inventory := ""
		if len(row.inventory) == 1 {
			inventory = row.inventory[0].Name
			for _, a := range row.inventory[0].Args {
				inventory += " " + a
			}
		}
		got := columns{row.kinds, row.bundleLine, row.install != nil, row.uninstall != nil, row.upgradable(), inventory, row.needs}
		w := want[m]
		if !slices.Equal(got.kinds, w.kinds) || got.bundle != w.bundle || got.install != w.install || got.uninstall != w.uninstall || got.upgrade != w.upgrade || got.inventory != w.inventory || got.needs != w.needs {
			t.Errorf("%s: row %+v, section c says %+v", m, got, w)
		}
	}
	if len(table) != len(want) {
		t.Errorf("%d rows for %d methods", len(table), len(want))
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
