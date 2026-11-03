package local

import (
	"context"
	"errors"
	"slices"
	"strconv"
	"strings"

	"github.com/jterrazz/studio-station/src/system"
)

type (
	Cmd    = system.Cmd
	Output = system.Output
	Runner = system.Runner
	FS     = system.FS
	Net    = system.Net
	Stream = system.Stream
)

const ToStderr = system.ToStderr

var ErrNotFound = system.ErrNotFound

func Command(name string, args ...string) Cmd { return Cmd{Name: name, Args: args} }

func Attach(ctx context.Context, r Runner, c Cmd) error {
	// invariant: a command attached to the owner's terminal fails by its name and
	// first word: "bun install exited 1", or "bun: <why>" when it could not run.
	ran, err := r.Attach(ctx, c, ToStderr)
	switch {
	case err != nil:
		return errors.New(c.Name + ": " + err.Error())
	case ran.ExitCode != 0:
		return &ExitError{Step: c.Name + " " + first(c.Args), Code: ran.ExitCode, LastStderr: ran.LastStderr}
	}
	return nil
}

type ExitError struct {
	Step       string
	Code       int
	LastStderr string
}

func (e *ExitError) Error() string { return e.Step + " exited " + strconv.Itoa(e.Code) }

func first(args []string) string {
	if len(args) == 0 {
		return ""
	}
	return args[0]
}

func Played(play func(c Cmd) (Output, error), names ...string) Runner {
	// value: a Runner answered by a function, as an adapter's test plays its
	// tool: a name it answers for is on PATH, any other is not found.
	return played{play: play, names: names}
}

type played struct {
	play  func(Cmd) (Output, error)
	names []string
}

func (p played) on(name string) bool { return slices.Contains(p.names, name) }

func (p played) Output(_ context.Context, c Cmd) (Output, error) {
	if !p.on(c.Name) {
		return Output{}, ErrNotFound
	}
	return p.play(c)
}

func (p played) Attach(ctx context.Context, c Cmd, _ system.Stream) (system.Attached, error) {
	out, err := p.Output(ctx, c)
	last := ""
	for _, line := range strings.Split(out.Stderr, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			last = line
		}
	}
	return system.Attached{ExitCode: out.ExitCode, LastStderr: last}, err
}

func (p played) Start(Cmd, string) (int, error) { return 0, errors.ErrUnsupported }

func (p played) LookPath(name string) (string, bool) {
	if !p.on(name) {
		return "", false
	}
	return "/played/" + name, true
}

func (p played) Alive(int) bool               { return false }
func (p played) Signal(int, system.Sig) error { return errors.ErrUnsupported }
func (p played) Reap(string) int              { return 0 }
