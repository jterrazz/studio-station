package wire

import (
	"context"
	"strings"

	"github.com/jterrazz/studio-station/src/page"
	"github.com/jterrazz/studio-station/src/surface/cli/kit"
	"github.com/jterrazz/studio-station/src/surface/tui"
	"github.com/jterrazz/studio-station/src/system"
)

func tuiOpener(_ kit.Env, m machine, noColor bool) func(context.Context) error {
	return func(ctx context.Context) error {
		return tui.Run(ctx, tui.Deps{Here: m.where, Pages: tuiPages(m), NoColor: noColor, Stop: m.pool.Close})
	}
}

// invariant: a screen is the page its command prints: the TUI runs this very
// binary for the CLI's words with --page, so a page has one function.
func tuiPages(m machine) tui.Pages {
	return func(ctx context.Context, words []string) (page.Page, error) {
		out, err := m.host.Run.Output(ctx, system.Cmd{Name: m.host.Env.Executable(), Args: append(words, "--page")})
		if err != nil {
			return page.Page{}, err
		}
		if out.ExitCode != 0 {
			return page.Page{}, failed(out.Stderr)
		}
		return page.Decode([]byte(out.Stdout))
	}
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
