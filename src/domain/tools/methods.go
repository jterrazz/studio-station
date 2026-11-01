package tools

import (
	"encoding/json"
	"strconv"
	"strings"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/system"
)

// invariant: THE method table. One row per catalog.Method, and the one type switch
// over the Install variants (brewfileLine and the builders below); no other file
// dispatches on a method or an install type.
type method struct {
	kinds      []catalog.MachineKind
	bundleLine bool
	install    func(catalog.Install) []system.Cmd
	uninstall  func(catalog.Install) []system.Cmd
	upgrade    func(catalog.Install) []system.Cmd
	inventory  []system.Cmd
	parse      func(stdout string) (map[string]string, error)
	manager    string
	needs      string
	// outside: `npm ls -g` exits 1 on an extraneous or missing peer while its JSON
	// still lists every global package.
	anyExit bool
}

func (m method) upgradable() bool { return m.bundleLine || m.upgrade != nil }

func methods() map[catalog.Method]method {
	bundled := func(m catalog.Method, uninstall func(catalog.Install) []system.Cmd, inventory system.Cmd, parse func(string) (map[string]string, error), manager string) method {
		return method{kinds: catalog.MethodKinds[m], bundleLine: true, uninstall: uninstall, inventory: []system.Cmd{inventory}, parse: parse, manager: manager, needs: "homebrew"}
	}
	return map[catalog.Method]method{
		catalog.MethodBrew: bundled(catalog.MethodBrew, uninstallArgv, cmd("brew", "list", "--formula", "--versions"), parseBrewList, "brew"),
		catalog.MethodCask: bundled(catalog.MethodCask, uninstallArgv, cmd("brew", "list", "--cask", "--versions"), parseBrewList, "brew"),
		catalog.MethodMas:  bundled(catalog.MethodMas, uninstallArgv, cmd("mas", "list"), parseMasList, "mas"),
		catalog.MethodNpm: func() method {
			m := bundled(catalog.MethodNpm, uninstallArgv, cmd("npm", "ls", "-g", "--depth=0", "--json"), parseNpmList, "npm")
			m.anyExit = true
			return m
		}(),
		catalog.MethodUv:     bundled(catalog.MethodUv, uninstallArgv, cmd("uv", "tool", "list"), parseUvList, "uv"),
		catalog.MethodBun:    {kinds: catalog.MethodKinds[catalog.MethodBun], install: installArgv, uninstall: uninstallArgv, upgrade: upgradeArgv, inventory: []system.Cmd{cmd("bun", "pm", "ls", "-g")}, parse: parseBunList, manager: "bun"},
		catalog.MethodMise:   {kinds: catalog.MethodKinds[catalog.MethodMise], install: installArgv, uninstall: uninstallArgv, upgrade: upgradeArgv, inventory: []system.Cmd{cmd("mise", "ls", "--global", "--json")}, parse: parseMiseList, manager: "mise", needs: "mise"},
		catalog.MethodScript: {kinds: catalog.MethodKinds[catalog.MethodScript], install: installArgv, upgrade: installArgv},
		catalog.MethodShell:  {kinds: catalog.MethodKinds[catalog.MethodShell], install: installArgv},
		catalog.MethodApp:    {kinds: catalog.MethodKinds[catalog.MethodApp]},
		catalog.MethodSkill:  {kinds: catalog.MethodKinds[catalog.MethodSkill], install: installArgv, uninstall: uninstallArgv, upgrade: installArgv, needs: "skills"},
	}
}

// hazard: the installer must keep the terminal as its stdin: Homebrew's install.sh
// turns NONINTERACTIVE and `sudo -n` when stdin is not a TTY, which is why its
// documented form is `bash -c "$(curl ...)"` rather than `curl | bash`.
const curlInstaller = `script="$(curl -fsSL "$0")" && exec bash -c "$script" bash "$@"`

func installArgv(i catalog.Install) []system.Cmd {
	switch v := i.(type) {
	case catalog.BunInstall:
		return []system.Cmd{cmd("bun", "install", "-g", v.Package)}
	case catalog.MiseInstall:
		return []system.Cmd{cmd("mise", "use", "--global", v.Tool+"@"+v.Version)}
	case catalog.ScriptInstall:
		c := cmd("bash", append([]string{"-c", curlInstaller, v.URL}, v.Args...)...)
		c.Env = v.Env
		return []system.Cmd{c}
	case catalog.ShellInstall:
		return []system.Cmd{cmd("bash", "-c", v.Line)}
	case catalog.SkillInstall:
		return []system.Cmd{cmd("skills", append([]string{"add", v.Repo, "-g", "-y", "--skill", v.Skill, "--agent"}, catalog.SkillAgents...)...)}
	}
	return nil
}

func uninstallArgv(i catalog.Install) []system.Cmd {
	switch v := i.(type) {
	case catalog.BrewInstall:
		return []system.Cmd{cmd("brew", "uninstall", v.Formula)}
	case catalog.CaskInstall:
		return []system.Cmd{cmd("brew", "uninstall", "--cask", v.Token)}
	case catalog.MasInstall:
		return []system.Cmd{cmd("mas", "uninstall", strconv.Itoa(v.ID))}
	case catalog.NpmInstall:
		return []system.Cmd{cmd("npm", "uninstall", "-g", v.Package)}
	case catalog.UvInstall:
		return []system.Cmd{cmd("uv", "tool", "uninstall", v.Key())}
	case catalog.BunInstall:
		return []system.Cmd{cmd("bun", "remove", "-g", v.Key())}
	case catalog.MiseInstall:
		return []system.Cmd{cmd("mise", "unuse", "--global", v.Tool)}
	case catalog.SkillInstall:
		return []system.Cmd{cmd("skills", "remove", "-g", "-y", v.Skill)}
	}
	return nil
}

func upgradeArgv(i catalog.Install) []system.Cmd {
	switch v := i.(type) {
	case catalog.BunInstall:
		return []system.Cmd{cmd("bun", "update", "-g", v.Key())}
	case catalog.MiseInstall:
		return []system.Cmd{cmd("mise", "upgrade", v.Tool)}
	}
	return nil
}

func brewfileLine(i catalog.Install) string {
	trusted := func(line string, t bool) string {
		if t {
			return line + ", trusted: true"
		}
		return line
	}
	switch v := i.(type) {
	case catalog.BrewInstall:
		return trusted(`brew "`+v.Formula+`"`, v.Trusted)
	case catalog.CaskInstall:
		return trusted(`cask "`+v.Token+`"`, v.Trusted)
	case catalog.MasInstall:
		return `mas "` + v.Name + `", id: ` + strconv.Itoa(v.ID)
	case catalog.NpmInstall:
		return `npm "` + v.Package + `"`
	case catalog.UvInstall:
		return `uv "` + v.Package + `"`
	}
	return ""
}

func cmd(name string, args ...string) system.Cmd { return system.Cmd{Name: name, Args: args} }

func parseBrewList(stdout string) (map[string]string, error) {
	out := map[string]string{}
	for _, line := range strings.Split(stdout, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		out[fields[0]] = fields[len(fields)-1]
		if len(fields) == 1 {
			out[fields[0]] = ""
		}
	}
	return out, nil
}

func parseMasList(stdout string) (map[string]string, error) {
	out := map[string]string{}
	for _, line := range strings.Split(stdout, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		version := ""
		if open := strings.LastIndex(line, "("); open >= 0 {
			version = strings.TrimSuffix(strings.TrimSpace(line[open+1:]), ")")
		}
		out[fields[0]] = version
	}
	return out, nil
}

func parseNpmList(stdout string) (map[string]string, error) {
	var doc struct {
		Dependencies map[string]struct {
			Version string `json:"version"`
		} `json:"dependencies"`
	}
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		return nil, err
	}
	out := map[string]string{}
	for name, d := range doc.Dependencies {
		out[name] = d.Version
	}
	return out, nil
}

func parseUvList(stdout string) (map[string]string, error) {
	out := map[string]string{}
	for _, line := range strings.Split(stdout, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 || strings.HasPrefix(fields[0], "-") {
			continue
		}
		version := ""
		if len(fields) > 1 {
			version = strings.TrimPrefix(fields[1], "v")
		}
		out[fields[0]] = version
	}
	return out, nil
}

func parseBunList(stdout string) (map[string]string, error) {
	out := map[string]string{}
	for _, line := range strings.Split(stdout, "\n") {
		entry := strings.TrimLeft(line, "├└│─ \t")
		if entry == line || entry == "" {
			continue
		}
		at := strings.LastIndex(entry, "@")
		if at <= 0 {
			out[entry] = ""
			continue
		}
		out[entry[:at]] = entry[at+1:]
	}
	return out, nil
}

func parseMiseList(stdout string) (map[string]string, error) {
	var doc map[string][]struct {
		Version   string `json:"version"`
		Installed bool   `json:"installed"`
	}
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		return nil, err
	}
	out := map[string]string{}
	for name, versions := range doc {
		for _, v := range versions {
			if v.Installed {
				out[name] = v.Version
				break
			}
		}
	}
	return out, nil
}
