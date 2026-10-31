package remote_test

import (
	"slices"
	"testing"

	"github.com/jterrazz/studio-station/src/domain/remote"
)

func TestWordsSurviveTheRemoteShell(t *testing.T) {
	for word, want := range map[string]string{
		"--json":       "--json",
		"tools":        "tools",
		"a b":          `'a b'`,
		"it's":         `'it'\''s'`,
		`say "hi"`:     `'say "hi"'`,
		"$HOME":        `'$HOME'`,
		"a;rm -rf /":   `'a;rm -rf /'`,
		"":             `''`,
		"feat: add x":  `'feat: add x'`,
		"a`b`":         "'a`b`'",
		"path/to-f.go": "path/to-f.go",
	} {
		if got := remote.Quote(word); got != want {
			t.Errorf("Quote(%q) = %s, want %s", word, got, want)
		}
	}
}

func TestTheArgvKeepsTheScriptOneWordAndAsksForATerminalOnlyWhenInteractive(t *testing.T) {
	want := []string{"x", "--", "sh", "-c", `'PATH="$HOME/.local/bin:/usr/local/bin:$PATH" exec station "$@"'`, "station", "tools", "--json"}
	if got := remote.Argv("x", false, []string{"tools", "--json"}); !slices.Equal(got, want) {
		t.Errorf("Argv = %q, want %q", got, want)
	}
	if got := remote.Argv("x", true, nil); got[0] != "-t" || got[1] != "x" {
		t.Errorf("interactive Argv = %q, want -t first", got)
	}
}

func TestOnAndItsValueLeaveTheWords(t *testing.T) {
	for _, c := range []struct{ in, want []string }{
		{[]string{"tools", "--on", "x", "--json"}, []string{"tools", "--json"}},
		{[]string{"--on=x", "tools"}, []string{"tools"}},
		{[]string{"--on", "x"}, []string{}},
		{[]string{"run", "git", "--", "--on", "x"}, []string{"run", "git", "--", "--on", "x"}},
	} {
		if got := remote.Words(c.in); !slices.Equal(got, c.want) {
			t.Errorf("Words(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
