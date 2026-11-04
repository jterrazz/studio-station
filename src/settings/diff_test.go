package settings_test

import (
	"testing"

	"github.com/jterrazz/studio-station/src/settings"
)

func TestUnifiedDiffShowsTheLineYouChangedAgainstStationsWithItsContext(t *testing.T) {
	want := "a\nb\nc\nd\ne\nf\ng\n"
	have := "a\nb\nc\nD\ne\nf\ng\n"
	got := settings.UnifiedDiff(want, have, "station", "~/x")
	expect := "--- station\n+++ ~/x\n@@ -2,5 +2,5 @@\n b\n c\n-d\n+D\n e\n f\n"
	if got != expect {
		t.Fatalf("diff =\n%s\nwant\n%s", got, expect)
	}
}

func TestUnifiedDiffIsEmptyWhenNothingChanged(t *testing.T) {
	if got := settings.UnifiedDiff("a\n", "a\n", "station", "~/x"); got != "" {
		t.Fatalf("diff = %q", got)
	}
}

func TestUnifiedDiffOfAMissingFileRemovesEveryLine(t *testing.T) {
	got := settings.UnifiedDiff("a\nb\n", "", "station", "~/x")
	if got != "--- station\n+++ ~/x\n@@ -1,2 +1,0 @@\n-a\n-b\n" {
		t.Fatalf("diff =\n%s", got)
	}
}
