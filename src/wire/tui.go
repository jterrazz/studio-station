package wire

import (
	"context"
	"strings"

	"github.com/jterrazz/studio-station/src/engine"
	"github.com/jterrazz/studio-station/src/engine/process"
	"github.com/jterrazz/studio-station/src/page"
	"github.com/jterrazz/studio-station/src/surface/cli/kit"
	"github.com/jterrazz/studio-station/src/surface/tui"
	"github.com/jterrazz/studio-station/src/system"
)

func tuiOpener(_ kit.Env, m machine, noColor bool) func(context.Context) error {
	return func(ctx context.Context) error {
		if err := m.world.lend(ctx); err != nil {
			return err
		}
		return tui.Run(ctx, tui.Deps{Here: m.where, Pages: tuiPages(m), Run: tuiCalls(m), NoColor: noColor, Stop: m.pool.Close, Watch: tuiWatch(ctx, m)})
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

// invariant: a key's call is the CLI's command for its words, run as this
// very binary, so a call has one function on every surface.
func tuiCalls(m machine) tui.Calls {
	return func(ctx context.Context, words []string) (string, error) {
		out, err := m.host.Run.Output(ctx, system.Cmd{Name: m.host.Env.Executable(), Args: words})
		if err != nil {
			return "", err
		}
		if out.ExitCode != 0 {
			return "", failed(out.Stderr)
		}
		return out.Stdout, nil
	}
}

// invariant: the TUI's watch is the menu bar's: engine.watch held open on
// this Mac's engine for the machines it shows, This Mac by the engine's own
// name for it, until the screen leaves them.
func tuiWatch(ctx context.Context, m machine) func([]string) func() {
	return func(names []string) func() {
		held := make([]string, 0, len(names))
		for _, n := range names {
			if n == m.where.Machine {
				n = engine.ThisMac
			}
			held = append(held, n)
		}
		if m.world != nil {
			return m.world.watch(held)
		}
		watch, cancel := context.WithCancel(ctx)
		done := make(chan struct{})
		go func() {
			defer close(done)
			updates, err := m.engineAPI().c.Watch(watch, process.Watch, process.WatchIn{Machines: held})
			if err != nil {
				return
			}
			for range updates {
			}
		}()
		return func() {
			cancel()
			<-done
		}
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
