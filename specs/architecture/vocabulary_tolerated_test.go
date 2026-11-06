package architecture_test

// invariant: the allowlist of rule vocabulary: each folder that still says a
// word SPEC §2.2 retired, and the words, grouped by the lot that retires
// them. It only shrinks; a word a folder no longer says fails the gate until
// its line is removed. A hidden alias, a legacy encoding (legacyEncodings)
// and another program's own text are not on it: the rule reads past them.
func vocabularyTolerated() map[string][]string {
	return map[string][]string{
		// value: the docs, which L58 rewrites in v3's words.
		"docs":           {"config", "daemon", "fleet", "host", "lasting", "lease", "publish", "service", "temporary", "tools"},
		"docs/concepts":  {"NAT", "config", "fleet", "host", "lasting", "lease", "lend", "place", "probe", "publish", "reset", "sandbox", "service", "storage", "stray", "temporary", "tool", "tools"},
		"docs/guides":    {"a check", "fleet", "host", "place", "power", "probe", "publish", "sandbox", "service", "tool", "tools"},
		"docs/reference": {"NAT", "OrbStack", "a check", "config", "daemon", "fleet", "host", "lease", "lend", "place", "power", "probe", "publish", "service", "storage", "stray", "tool", "tools", "wish"},
		"docs/start":     {"clean", "config", "host", "lend", "tools"},
		// value: the old CLI suite, kept untouched (PLAN rule 4) until its
		// documents move.
		"specs/cli/daemon":          {"daemon", "lasting", "lease", "temporary"},
		"specs/cli/guest":           {"host", "publish"},
		"specs/cli/machines":        {"fleet", "host", "stray", "temporary"},
		"specs/cli/machines/create": {"NAT", "daemon", "host", "lasting", "service", "temporary"},
		"specs/cli/machines/delete": {"temporary"},
		"specs/cli/machines/pipe":   {"NAT", "host", "lasting", "probe"},
		"specs/cli/machines/start":  {"daemon", "lasting"},
		"specs/cli/machines/stop":   {"daemon", "lasting"},
		"specs/cli/on":              {"host", "probe"},
		// value: src/domain and src/system, which L56 deletes, and the code
		// that still speaks them: wire's embedded domain services, the hello
		// the domain guest reads (src/engine), the domain machines.Request the
		// menu bar's form fills (Resources).
		"src/pages/menubar": {"Resources"},
		// value: machines.probe and its hidden alias `machines probe`, which
		// L56 removes.
		"src/surface/cli/machines/probe": {"probe"},
		// value: the reserved ports' file, station.leases/v1, until its store
		// migrates to reservations.json (ARCHITECTURE §12).
		"src/ports/adapter/jsonfile": {"lease"},
		// value: the code src/domain held, moved into the families by L56 with
		// the words it still says; L56 sweeps it.
		"src/accounts":                  {"clean", "lend"},
		"src/accounts/calls":            {"probe", "recall"},
		"src/apps/calls":                {"config", "probe", "service", "tools"},
		"src/awake":                     {"daemon", "host", "power"},
		"src/awake/calls":               {"reset", "service"},
		"src/catalog":                   {"config", "host", "power"},
		"src/disk/calls":                {"probe", "reset", "service", "storage"},
		"src/engine/process":            {"host", "power"},
		"src/images":                    {"publish"},
		"src/images/calls":              {"clean", "service"},
		"src/machines":                  {"host", "power"},
		"src/machines/calls":            {"config", "daemon", "host", "service"},
		"src/platform/local":            {"host"},
		"src/ports":                     {"host", "lease"},
		"src/ports/adapter/docker":      {"publish"},
		"src/ports/adapter/guest":       {"host", "publish", "service"},
		"src/ports/calls":               {"host", "lease", "service"},
		"src/settings":                  {"config", "place"},
		"src/settings/adapter/zshblock": {"service"},
		"src/setup":                     {"config"},
		"src/setup/calls":               {"service", "stray"},
		"src/surface/cli/kit":           {"daemon"},
		"src/wire":                      {"config", "host", "probe", "service"},
	}
}

// invariant: the legacy encodings, each a declaration marked "value: alias
// of": station.status/v1 (the OS connector's, byte for byte one release),
// its rows and its kind word; the v2 keys a store reads once and writes in
// v3's words; v2's hidden commands that are no kit.Alias.
func legacyEncodings() []string {
	return []string{
		"src/accounts/adapter/jsonfile sharedIn",
		"src/accounts/contract GroupV2ShareList",
		"src/catalog StatusV1Kind",
		"src/images UnmarshalJSON",
		"src/machines UnmarshalText",
		"src/machines/adapter/jsonfile file",
		"src/pages/apps AppStatusJSON",
		"src/pages/apps statusV1Category",
		"src/pages/home SnapshotJSON",
		"src/pages/home awakeJSON",
		"src/pages/home stationJSON",
		"src/pages/home workspaceJSON",
		"src/pages/settings ItemJSON",
		"src/ports Reservation",
		"src/ports/contract RetiredGroupPorts",
		"src/setup/contract RetiredGroupName",
		"src/surface/cli/kit RetiredEngineCommand",
		"src/surface/cli/settings containerShortcuts",
		"src/world Kept",
	}
}
