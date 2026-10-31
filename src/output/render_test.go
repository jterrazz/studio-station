package output_test

import (
	"bytes"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/jterrazz/studio-station/src/fail"
	"github.com/jterrazz/studio-station/src/output"
	"github.com/jterrazz/studio-station/src/present"
	"github.com/jterrazz/studio-station/src/theme"
)

var update = flag.Bool("update", false, "rewrite the goldens from the renderer")

func TestTheTTYRendererReproducesTheMockups(t *testing.T) {
	for name, doc := range map[string]present.Document{"status": statusDocument(), "tools": toolsDocument()} {
		t.Run(name, func(t *testing.T) {
			// invariant: the dark-80 golden IS the SPEC mockup, so -update never rewrites it.
			if got, want := ansi.Strip(output.TTYString(doc, 80, theme.New(true))), read(t, name+".dark80.golden"); got != want {
				t.Errorf("dark 80 columns, stripped of its color, differs from the SPEC mockup:\n%s\nwant:\n%s", got, want)
			}
			golden(t, name+".light80.golden", output.TTYString(doc, 80, theme.New(false)))
			golden(t, name+".dark40.golden", ansi.Strip(output.TTYString(doc, 40, theme.New(true))))
		})
	}
}

func TestTheErrorBlockOnATerminal(t *testing.T) {
	var stderr bytes.Buffer
	w := output.NewWriter(output.Stream{}, output.Stream{W: &stderr, IsTerminal: true, Dark: true},
		map[string]string{"CLICOLOR_FORCE": "1", "COLORTERM": "truecolor", "TERM": "xterm-256color"})
	w.Failure(&fail.Error{Message: `unknown tool "nope"`, Hint: "run station tools for the catalogue", Code: fail.CodeUsage})
	golden(t, "error.tty.golden", stderr.String())

	stderr.Reset()
	w.Failure(&fail.Error{Code: fail.CodeUsage})
	w.Failure(&fail.Error{Code: fail.CodeInterrupted})
	if stderr.Len() != 0 {
		t.Fatalf("a silent or interrupted error printed %q, want nothing", stderr.String())
	}
	w.Failure(errors.New("boom"))
	if !bytes.HasPrefix([]byte(ansi.Strip(stderr.String())), []byte("Error: boom\n")) {
		t.Fatalf("an unworded error printed %q", stderr.String())
	}
}

func read(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func golden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", name)
	if *update {
		if err := os.WriteFile(path, []byte(got), 0o600); err != nil {
			t.Fatal(err)
		}
		return
	}
	if want := read(t, name); got != want {
		t.Errorf("%s differs (go test ./src/output/ -update rewrites it):\n%s\nwant:\n%s", name, got, want)
	}
}

func row(mark present.Mark, cells ...present.Cell) present.Row {
	return present.Row{Mark: mark, Cells: cells}
}

func name(s string) present.Cell   { return present.Cell{Text: s, Tone: present.Strong} }
func value(s string) present.Cell  { return present.Cell{Text: s} }
func detail(s string) present.Cell { return present.Cell{Text: s, Tone: present.Muted} }

func section(title string, counts []present.Count, rows ...present.Row) present.Section {
	return present.Section{Title: title, Counts: counts, Blocks: []present.Block{present.Table{Rows: rows}}}
}

func counts(ok, degraded, absent, broken int) []present.Count {
	return []present.Count{
		{Mark: present.MarkBroken, N: broken},
		{Mark: present.MarkAbsent, N: absent},
		{Mark: present.MarkDegraded, N: degraded},
		{Mark: present.MarkOK, N: ok},
	}
}

func statusDocument() present.Document {
	ok, degraded, absent := present.MarkOK, present.MarkDegraded, present.MarkAbsent
	return present.Document{
		Command: "station status",
		Context: present.MachineContext("macbook", "host", true),
		Blocks: []present.Block{
			section("Machine", nil,
				row(ok, name("name"), value("macbook"), detail("host · Darwin 27.0.0 arm64 · jterrazz · zsh")),
				row(ok, name("uptime"), value("3d 4h"), detail("battery 92% charging")),
				row(ok, name("setup"), value("done")),
			),
			section("Environment", nil,
				row(ok, name("local ip"), value("192.168.1.24")),
				row(ok, name("public ip"), value("82.65.10.4")),
				row(absent, name("vpn"), value("off")),
				row(ok, name("filevault"), value("on")),
				row(degraded, name("brew"), value("3 outdated"), detail("station tools upgrade --via brew")),
				row(ok, name("tailnet"), value("connected"), detail("userspace · 100.64.0.3")),
			),
			section("Workspace", nil,
				row(ok, name("repos"), value("41"), detail("3 dirty")),
				row(degraded, name("deps"), value("7 outdated"), detail("in 3 repos")),
				row(ok, name("docker"), value("running"), detail("4 containers · 12 images")),
				row(ok, name("disk"), value("18.4 GB"), detail("reclaimable: station clean")),
			),
			section("Identity", nil,
				row(ok, name("git email"), value("set"), detail("admin@jterrazz.com")),
				row(ok, name("ssh key"), value("present"), detail("id_ed25519")),
			),
			section("Tools", counts(58, 2, 14, 0),
				row(degraded, name("node"), value("24.11.0"), detail("on PATH, not managed by mise")),
				row(absent, name("mise"), value(""), detail("station tools install mise")),
			),
			section("Config", counts(9, 0, 3, 0),
				row(absent, name("hushlogin"), value(""), detail("station config install hushlogin")),
			),
		},
		Hints: []string{"station tools for the catalogue", "station config for the items"},
	}
}

func toolsDocument() present.Document {
	ok, degraded, absent, broken := present.MarkOK, present.MarkDegraded, present.MarkAbsent, present.MarkBroken
	return present.Document{
		Command: "station tools",
		Context: present.MachineContext("macbook", "host", true),
		Blocks: []present.Block{
			section("Package Managers", counts(3, 0, 0, 0),
				row(ok, name("homebrew"), value("5.0.1"), detail("script"), detail("312 formulae, 98 casks")),
				row(ok, name("mise"), value("2026.10"), detail("brew")),
				row(ok, name("uv"), value("0.9.2"), detail("brew")),
			),
			section("Runtimes", counts(4, 1, 1, 0),
				row(degraded, name("node"), value("24.11.0"), detail("mise"), detail("on PATH, not managed by mise")),
				row(ok, name("go"), value("1.27.3"), detail("mise")),
				row(absent, name("java"), value(""), detail("mise"), detail("station tools install java")),
			),
			section("AI Agents", counts(9, 0, 0, 1),
				row(ok, name("claude"), value("2.1.4"), detail("script")),
				row(broken, name("hermes"), value(""), detail("script"), detail("hermes --version exited 1")),
			),
		},
		Hints: []string{"station tools install <tool...>", "station tools upgrade"},
	}
}
