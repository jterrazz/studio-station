package wire

import (
	"context"
	"regexp"
	"strings"
	"sync"

	"github.com/jterrazz/studio-station/src/page"
	"github.com/jterrazz/studio-station/src/surface/cli/kit"
	"github.com/jterrazz/studio-station/src/surface/tui"
	"github.com/jterrazz/studio-station/src/system"
	"github.com/jterrazz/studio-station/src/ui/token"
)

func tuiOpener(_ kit.Env, m machine, noColor bool) func(context.Context) error {
	return func(ctx context.Context) error {
		if m.world != nil {
			m.world.standAside()
		}
		b := binary{m: m, world: m.world != nil}
		return tui.Run(ctx, tui.Deps{Here: m.where, Pages: b.pages, Do: b.do, Open: b.open, Terminal: b.terminal, NoColor: noColor, Stop: m.pool.Close})
	}
}

// invariant: a screen is the page its command prints and an action the
// command it runs: the TUI runs this very binary for the CLI's words, so a
// page and a gesture each have one function.
type binary struct {
	m machine
	// value: in a spec world every run reads the world at its start and
	// writes it back at its end, so an action runs alone: a read that
	// started before it would write its effect away.
	world bool
	turns sync.RWMutex
}

func (b *binary) pages(ctx context.Context, words []string) (page.Page, error) {
	if b.world {
		b.turns.RLock()
		defer b.turns.RUnlock()
	}
	out, err := b.run(ctx, append(words, "--page"))
	if err != nil {
		return page.Page{}, err
	}
	if out.ExitCode != 0 {
		return page.Page{}, failed(out.Stderr)
	}
	return page.Decode([]byte(out.Stdout))
}

var typed = regexp.MustCompile(`--confirm=(\S+)`)

func (b *binary) do(ctx context.Context, words []string, a tui.Answer) error {
	if b.world {
		b.turns.Lock()
		defer b.turns.Unlock()
	}
	switch {
	case a.Typed != "":
		words = append(words, "--confirm="+a.Typed)
	case a.Yes:
		words = append(words, "--yes")
	}
	out, err := b.run(ctx, words)
	if err != nil {
		return err
	}
	if out.ExitCode == 0 {
		return nil
	}
	f := failed(out.Stderr)
	if why := broken(out.Stdout); why != "" {
		f.Message = why
	}
	if out.ExitCode == 2 && !a.Yes && a.Typed == "" {
		if name := typed.FindStringSubmatch(f.Hint); name != nil {
			return &tui.Asked{Message: f.Message, Name: name[1]}
		}
		if strings.Contains(f.Hint, "--yes") {
			return &tui.Asked{Message: f.Message}
		}
	}
	return f
}

func (b *binary) run(ctx context.Context, args []string) (system.Output, error) {
	return b.m.host.Run.Output(ctx, system.Cmd{Name: b.m.host.Env.Executable(), Args: args})
}

func (b *binary) open(ctx context.Context, target string) error {
	return b.exec(ctx, system.Cmd{Name: "open", Args: []string{target}})
}

// invariant: a new terminal is Terminal.app told to run the line, as the
// menu bar's Open in Terminal does.
func (b *binary) terminal(ctx context.Context, line string) error {
	quoted := strings.ReplaceAll(strings.ReplaceAll(line, `\`, `\\`), `"`, `\"`)
	return b.exec(ctx, system.Cmd{Name: "osascript", Args: []string{"-e", `tell application "Terminal" to do script "` + quoted + `"`, "-e", `tell application "Terminal" to activate`}})
}

func (b *binary) exec(ctx context.Context, c system.Cmd) error {
	out, err := b.m.host.Run.Output(ctx, c)
	if err != nil {
		return err
	}
	if out.ExitCode != 0 {
		return failed(out.Stderr)
	}
	return nil
}

// value: why the one record a command printed broken failed, in its own
// words (the install's log), rather than the run's summing-up line.
func broken(stdout string) string {
	var why []string
	for _, line := range strings.Split(stdout, "\n") {
		if strings.HasPrefix(line, token.MarkBroken+"\t") {
			fields := strings.Split(line, "\t")
			why = append(why, strings.TrimSpace(fields[len(fields)-1]))
		}
	}
	if len(why) != 1 {
		return ""
	}
	return why[0]
}

func failed(stderr string) *tui.Failed {
	f := &tui.Failed{}
	for _, line := range strings.Split(strings.TrimSpace(stderr), "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "Error:"):
			f.Message = strings.TrimSpace(strings.TrimPrefix(line, "Error:"))
		case strings.HasPrefix(line, "Hint:"):
			f.Hint = strings.TrimSpace(strings.TrimPrefix(line, "Hint:"))
		case f.Message == "":
			f.Message = line
		}
	}
	return f
}
