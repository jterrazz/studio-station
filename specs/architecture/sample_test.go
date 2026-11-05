package architecture_test

import (
	"slices"
	"testing"
)

func samplePath() []string {
	return []string{"activity/adapter/macos", "activity/adapter/linux"}
}

func spawning() []string {
	return []string{"os/exec", module + "/src/platform/local", module + "/src/system"}
}

// invariant: the allowlist of no-subprocess-in-sample: the Meters that still
// run a tool per sample. It only shrinks; a line no longer true fails the gate
// until it is removed.
func sampleTolerated() []string {
	return nil
}

// invariant: rule no-subprocess-in-sample (13.3 rule 10, SPEC §6.3): the
// Meters the sample job reaches read the kernel's counters through
// platform/darwin or /proc, so no sample starts a process: they import no
// way to run one.
func ruleNoSubprocessInSample(w *world) []finding {
	var out []finding
	offends := map[string]bool{}
	for _, f := range w.files {
		if f.test || !slices.Contains(samplePath(), f.pkg) {
			continue
		}
		for i, ip := range f.imports {
			if !slices.Contains(spawning(), ip) {
				continue
			}
			offends[f.pkg] = true
			if !slices.Contains(sampleTolerated(), f.pkg) {
				out = append(out, finding{"no-subprocess-in-sample", f.at(f.syntax.Imports[i].Pos()), f.pkg + " samples without a process: it may not import " + ip})
			}
		}
	}
	for _, pkg := range sampleTolerated() {
		if w.packages[pkg] && !offends[pkg] {
			out = append(out, finding{"no-subprocess-in-sample", "specs/architecture/sample_test.go", pkg + " no longer runs a tool: remove its line"})
		}
	}
	return out
}

func TestNoSubprocessInSample(t *testing.T) { judge(t, "NoSubprocessInSample") }
