package tab

import (
	"context"
	"io"

	tea "charm.land/bubbletea/v2"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/core/call"
	"github.com/jterrazz/studio-station/src/core/confirm"
	"github.com/jterrazz/studio-station/src/page"
	"github.com/jterrazz/studio-station/src/pages"
	"github.com/jterrazz/studio-station/src/ui/render/tui"
)

type Screen interface {
	Init() tea.Cmd
	Update(msg tea.Msg) (Screen, tea.Cmd)
	Body(width, height int) string
	Footer(width int) string
	Modal() bool
	Busy() bool
	Loading() bool
	Owns(msg tea.Msg) bool
}

type Describe func(error) (message, hint string)

type Status struct {
	Mark       page.Mark
	Text, Hint string
}

type Reload struct{}

type Shown struct{ Visible bool }

type Done struct {
	Screen string
	Status Status
	Err    error
}

type ExecMsg struct {
	// invariant: a mutating action leaves its tab through this message only,
	// so the program is the one place that releases the terminal (tea.Exec).
	Command ExecFunc
	Done    func(error) tea.Msg
	// value: the action never reads or writes the terminal, so it runs with the
	// screen kept drawn.
	InPlace bool
}

type ExecFunc func() error

func (f ExecFunc) Run() error { return f() }

func (ExecFunc) SetStdin(io.Reader)  {}
func (ExecFunc) SetStdout(io.Writer) {}
func (ExecFunc) SetStderr(io.Writer) {}

func Exec(screen string, action func() (Status, error)) tea.Cmd {
	return run(screen, action, false)
}

func InPlace(screen string, action func() (Status, error)) tea.Cmd {
	return run(screen, action, true)
}

func run(screen string, action func() (Status, error), inPlace bool) tea.Cmd {
	var status Status
	var failure error
	return func() tea.Msg {
		return ExecMsg{
			Command: func() error {
				status, failure = action()
				return nil
			},
			Done: func(err error) tea.Msg {
				if err != nil {
					return Done{Screen: screen, Err: err}
				}
				return Done{Screen: screen, Status: status, Err: failure}
			},
			InPlace: inPlace,
		}
	}
}

type asked struct {
	id       string
	question page.Confirm
	then     tea.Cmd
}

func Confirmed(screen string, question page.Confirm, action func(answer string) (Status, error)) tea.Cmd {
	return confirmed(screen, question, action, false)
}

func ConfirmedInPlace(screen string, question page.Confirm, action func(answer string) (Status, error)) tea.Cmd {
	return confirmed(screen, question, action, true)
}

func confirmed(screen string, question page.Confirm, action func(answer string) (Status, error), inPlace bool) tea.Cmd {
	// invariant: the action runs unanswered first, so its use case decides
	// whether it asks; only then is the question put, and a yes runs it once
	// more with the answer the use case asked for.
	var status Status
	var failure error
	return func() tea.Msg {
		return ExecMsg{
			Command: func() error {
				status, failure = action("")
				return nil
			},
			Done: func(err error) tea.Msg {
				if err != nil {
					return Done{Screen: screen, Err: err}
				}
				if answer, ok := confirm.Answer(failure); ok {
					return asked{id: screen, question: question, then: run(screen, func() (Status, error) { return action(answer) }, inPlace)}
				}
				return Done{Screen: screen, Status: status, Err: failure}
			},
			InPlace: inPlace,
		}
	}
}

func Say(s Status) tea.Cmd { return func() tea.Msg { return s } }

func Outcome(done, name string, state catalog.State, detail string) Status {
	mark := pages.MarkOf(state)
	if state == catalog.StateBroken {
		return Status{Mark: mark, Text: name + " failed", Hint: detail}
	}
	text := done + " " + name
	if detail != "" {
		text += " " + detail
	}
	return Status{Mark: mark, Text: text}
}

func Failed(describe Describe, err error) Status {
	text, hint := describe(err)
	return Status{Mark: page.Broken, Text: text, Hint: hint}
}

type Source[T any] func(ctx context.Context, fresh bool) (<-chan T, error)

type View[T any] struct {
	ID       string
	Browser  tui.Browser
	Last     T
	source   Source[T]
	build    func(T) page.Page
	describe Describe
	here     page.Context
	loading  bool
	busy     bool
	gen      int
	// invariant: a hidden tab keeps its latest page unlaid until it is shown,
	// so answers for tabs nobody sees cost the program no layout.
	hidden bool
	held   *page.Page
	// invariant: Init runs on a copy, so the running watch's cancel is held
	// where every copy of the tab sees it.
	stop *context.CancelFunc
}

type loaded[T any] struct {
	id    string
	gen   int
	value T
	page  page.Page
	more  <-chan T
	final bool
}

func NewView[T any](id, title, noun string, here page.Context, source Source[T], build func(T) page.Page, describe Describe) View[T] {
	return View[T]{ID: id, Browser: tui.New(title, noun), source: source, build: build, describe: describe, here: here, loading: true, stop: new(context.CancelFunc)}
}

func (v View[T]) Init() tea.Cmd { return v.watch(false) }

func (v View[T]) watch(fresh bool) tea.Cmd {
	if *v.stop != nil {
		(*v.stop)()
	}
	ctx, stop := context.WithCancel(context.Background()) //nolint:gosec // outside: held in *v.stop, called when the next watch starts.
	*v.stop = stop
	id, gen, source, build, describe, here := v.ID, v.gen, v.source, v.build, v.describe, v.here
	return func() tea.Msg {
		ch, err := source(ctx, fresh)
		if err != nil {
			message, hint := describe(err)
			return loaded[T]{id: id, gen: gen, page: pages.Failure(page.ID(id), "", here, message, hint)}
		}
		return next(id, gen, build, ch)()
	}
}

// invariant: the values a watch sent while the last one was being drawn are
// read at once and only the latest is built, so a burst of answers is one
// message: the program's queue stays short and a key (q) is never behind it.
func next[T any](id string, gen int, build func(T) page.Page, ch <-chan T) tea.Cmd {
	return func() tea.Msg {
		value, open := <-ch
		if !open {
			return loaded[T]{id: id, gen: gen, final: true}
		}
		for drained := false; !drained; {
			select {
			case v, ok := <-ch:
				if !ok {
					drained = true
					break
				}
				value = v
			default:
				drained = true
			}
		}
		return loaded[T]{id: id, gen: gen, value: value, page: build(value), more: ch}
	}
}

func (v View[T]) Owns(msg tea.Msg) bool {
	// invariant: a watch's answer belongs to the one screen that asked for it;
	// every other message may concern any screen.
	l, answer := msg.(owned)
	return !answer || l.owner() == v.ID
}

type owned interface{ owner() string }

func (l loaded[T]) owner() string { return l.id }

func (v View[T]) Modal() bool {
	f := v.Browser.Focus()
	return f == tui.Modal || f == tui.Filter || f == tui.Sheet
}

func (v View[T]) Busy() bool { return v.busy }

func (v View[T]) Loading() bool { return v.loading }

func (v View[T]) Body(width, height int) string { return v.Browser.Body(width, height) }

func (v View[T]) Footer(width int) string { return v.Browser.Footer(width, v.busy) }

type start struct {
	id  string
	run tea.Cmd
}

func Begin(id string, run tea.Cmd) tea.Msg { return start{id: id, run: run} }

func (v View[T]) Act(run tea.Cmd) (View[T], tea.Cmd) {
	v.busy = true
	return v, run
}

func (v View[T]) Update(msg tea.Msg) (View[T], tea.Cmd, *tui.Event) {
	switch msg := msg.(type) {
	case loaded[T]:
		if msg.id != v.ID || msg.gen != v.gen {
			return v, nil, nil
		}
		if msg.final {
			v.loading = false
			return v, nil, nil
		}
		if msg.more == nil {
			v.loading = false
			v = v.show(msg.page)
			return v, nil, nil
		}
		v.Last = msg.value
		v = v.show(msg.page)
		return v, next(msg.id, msg.gen, v.build, msg.more), nil
	case Shown:
		v.hidden = !msg.Visible
		if !v.hidden && v.held != nil {
			v.Browser, v.held = v.Browser.Show(*v.held), nil
		}
		return v, nil, nil
	case Done:
		if msg.Screen != v.ID {
			return v, nil, nil
		}
		v.busy = false
		status := msg.Status
		if msg.Err != nil {
			status = Failed(v.describe, msg.Err)
		}
		v.loading = true
		v.gen++
		return v, tea.Batch(Say(status), v.watch(true)), nil
	case asked:
		if msg.id != v.ID {
			return v, nil, nil
		}
		v.busy = false
		id, then := v.ID, msg.then
		asking, cmd := v.ask(msg.question, func() tea.Msg { return start{id: id, run: then} })
		return asking, cmd, nil
	case start:
		if msg.id != v.ID {
			return v, nil, nil
		}
		v.busy = true
		return v, msg.run, nil
	case Reload:
		if v.busy {
			return v, nil, nil
		}
		v.loading = true
		v.gen++
		return v, v.watch(true), nil
	}
	var cmd tea.Cmd
	var ev *tui.Event
	v.Browser, cmd, ev = v.Browser.Update(msg)
	if v.busy {
		ev = nil
	}
	return v, cmd, ev
}

func (v View[T]) show(p page.Page) View[T] {
	if v.hidden {
		v.held = &p
		return v
	}
	v.Browser, v.held = v.Browser.Show(p), nil
	return v
}

func (v View[T]) ask(c page.Confirm, then tea.Cmd) (View[T], tea.Cmd) {
	if c.Type == "" {
		v.Browser = v.Browser.Ask(c, then)
		return v, nil
	}
	f := page.Form{Title: c.Question, Fields: []page.Field{{Key: "name", Label: "type " + c.Type + " to confirm", Kind: page.FieldText, Required: true}}}
	var cmd tea.Cmd
	v.Browser, cmd = v.Browser.Fill(f, func(values map[string]string) tea.Cmd {
		if values["name"] != c.Type {
			return nil
		}
		return then
	})
	return v, cmd
}

func (v View[T]) Fill(f page.Form, submit func(map[string]string) tea.Cmd) (View[T], tea.Cmd) {
	var cmd tea.Cmd
	v.Browser, cmd = v.Browser.Fill(f, submit)
	return v, cmd
}

func Values[T any](updates <-chan call.Update[T], err error) (<-chan T, error) {
	// value: a call's updates as the values a View reads; an update in
	// error draws nothing.
	if err != nil {
		return nil, err
	}
	out := make(chan T)
	go func() {
		defer close(out)
		for u := range updates {
			if u.Err == nil {
				out <- u.Value
			}
		}
	}()
	return out, nil
}
