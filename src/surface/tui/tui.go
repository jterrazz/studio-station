package tui

import (
	"context"

	tea "charm.land/bubbletea/v2"

	"github.com/jterrazz/studio-station/src/page"
)

type Pages func(ctx context.Context, words []string) (page.Page, error)

type Deps struct {
	Here page.Context
	// invariant: the TUI's one read is the page the CLI prints for these
	// words, so a screen is the same page function as its command.
	Pages Pages
	// value: runs station with the CLI's words, the call a key makes, and
	// answers what it printed.
	Run     Calls
	NoColor bool
	// value: stops every read still running, so q leaves nothing behind.
	Stop func()
}

func Run(ctx context.Context, d Deps) error {
	_, err := tea.NewProgram(newShell(ctx, d), tea.WithContext(ctx)).Run()
	if d.Stop != nil {
		d.Stop()
	}
	return err
}
