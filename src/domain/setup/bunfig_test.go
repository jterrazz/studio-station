package setup

import (
	"strings"
	"testing"
)

const (
	global = "/h/.local/share/bun/install/global"
	bin    = "/h/.local/share/bun/bin"
	cached = "/h/.cache/bun/install"
)

func TestTheBunfigBlockIsWrittenWholeAndAbsolute(t *testing.T) {
	got, changed, conflict := UpsertBunfig("", global, bin, cached)
	want := "# >>> station >>>\n[install]\nglobalDir = \"" + global + "\"\nglobalBinDir = \"" + bin + "\"\n\n[install.cache]\ndir = \"" + cached + "\"\n# <<< station <<<\n"
	if got != want || !changed || conflict != "" {
		t.Fatalf("got %q (changed %v, conflict %q)", got, changed, conflict)
	}
	again, changed, _ := UpsertBunfig("telemetry = false\n"+got, global, bin, cached)
	if changed || again != "telemetry = false\n"+got {
		t.Fatalf("a rerun changed the file: %q", again)
	}
}

func TestAnOwnersInstallTableIsLeftAlone(t *testing.T) {
	own := "[install]\nregistry = \"https://r.example/\"\n"
	got, changed, conflict := UpsertBunfig(own, global, bin, cached)
	if got != own || changed || !strings.Contains(conflict, "has its own [install]") {
		t.Fatalf("got %q (changed %v, conflict %q)", got, changed, conflict)
	}
}
