package kit

import (
	"io"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/present"
)

type Focus int

const (
	FocusNormal Focus = iota
	FocusFilter
	FocusModal
)

type Screen interface {
	Init() tea.Cmd
	Update(msg tea.Msg) (Screen, tea.Cmd)
	View(width, height int) string
	Focus() Focus
	Busy() bool
	Loading() bool
	Keys() []key.Binding
}

type Describe func(error) (message, hint string)

type Status struct {
	Mark       present.Mark
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
	// invariant: a mutating action leaves its screen through this message only,
	// so the App is the one place that releases the terminal (tea.Exec) for it.
	Command ExecFunc
	Done    func(error) tea.Msg
}

type ExecFunc func() error

func (f ExecFunc) Run() error { return f() }

func (ExecFunc) SetStdin(io.Reader)  {}
func (ExecFunc) SetStdout(io.Writer) {}
func (ExecFunc) SetStderr(io.Writer) {}

func Exec(screen string, action func() (Status, error)) tea.Cmd {
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
		}
	}
}

func Say(s Status) tea.Cmd { return func() tea.Msg { return s } }

func Outcome(done, name string, state catalog.State, detail string) Status {
	mark := present.MarkOf(state)
	if state == catalog.StateBroken {
		return Status{Mark: mark, Text: name + " failed", Hint: detail}
	}
	text := done + " " + name
	if detail != "" {
		text += " " + detail
	}
	return Status{Mark: mark, Text: text}
}
