package architecture_test

import (
	"slices"
	"testing"
)

func samplePath() []string {
	return []string{"activity/adapter/macos", "activity/adapter/linux"}
}

func spawning() []string {
	return []string{"os/exec", module + "/src/platform/local"}
}

// invariant: rule no-subprocess-in-sample (13.3 rule 10, SPEC §6.3): the
// Meters the sample job reaches read the kernel's counters through
// platform/darwin or /proc, so no sample starts a process: they import no
// way to run one.
func ruleNoSubprocessInSample(w *world) []finding {
	var out []finding
	for _, f := range w.files {
		if f.test || !slices.Contains(samplePath(), f.pkg) {
			continue
		}
		for i, ip := range f.imports {
			if !slices.Contains(spawning(), ip) {
				continue
			}
			out = append(out, finding{"no-subprocess-in-sample", f.at(f.syntax.Imports[i].Pos()), f.pkg + " samples without a process: it may not import " + ip})
		}
	}
	return out
}

func TestNoSubprocessInSample(t *testing.T) { judge(t, "NoSubprocessInSample") }
