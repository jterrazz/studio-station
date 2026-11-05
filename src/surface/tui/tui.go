package tui

import (
	"context"

	tea "charm.land/bubbletea/v2"

	"github.com/jterrazz/studio-station/src/page"
)

type Pages func(ctx context.Context, words []string) (page.Page, error)

type Answer struct {
	// value: how the owner answered the question a command asked: yes, or the
	// name typed; the zero Answer runs the command unanswered.
	Yes   bool
	Typed string
}

type Do func(ctx context.Context, words []string, a Answer) error

type Asked struct {
	// value: the command did nothing yet: it asks first, yes or no, or the name
	// typed when Name is set.
	Message, Name string
}

func (a *Asked) Error() string { return a.Message }

type Deps struct {
	Here page.Context
	// invariant: the TUI's one read is the page the CLI prints for these
	// words, so a screen is the same page function as its command.
	Pages Pages
	// invariant: the TUI's one gesture is the command the CLI runs for these
	// words: the call asks its own question, the TUI only puts it.
	Do Do
	// value: opens a URL or a file with its app (a System Settings pane).
	Open func(ctx context.Context, target string) error
	// value: a new terminal window runs this command line.
	Terminal func(ctx context.Context, line string) error
	NoColor  bool
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
