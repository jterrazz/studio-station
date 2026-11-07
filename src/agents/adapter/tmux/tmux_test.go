package tmux_test

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/jterrazz/studio-station/src/agents/adapter/tmux"
	"github.com/jterrazz/studio-station/src/agents/contract/contracttest"
	"github.com/jterrazz/studio-station/src/platform/local"
)

type server struct {
	// value: a recorded tmux server: its session runs once opened, its
	// buffer holds what was loaded until pasted.
	running        bool
	buffer         string
	lines, opened  []string
	typed, pending []string
}

func (s *server) Output(_ context.Context, c local.Cmd) (local.Output, error) {
	s.lines = append(s.lines, strings.Join(append([]string{c.Name}, c.Args...), " "))
	if c.Name != "tmux" || len(c.Args) == 0 {
		return local.Output{ExitCode: 127}, nil
	}
	arg := func(flag string) string {
		if i := slices.Index(c.Args, flag); i >= 0 && i+1 < len(c.Args) {
			return c.Args[i+1]
		}
		return ""
	}
	switch c.Args[0] {
	case "has-session":
		if !s.running || arg("-t") != "main" {
			return local.Output{ExitCode: 1, Stderr: "can't find session: main"}, nil
		}
	case "new-session", "new-window":
		if c.Args[0] == "new-window" && !s.running {
			return local.Output{ExitCode: 1}, nil
		}
		s.running = true
		s.opened = append(s.opened, arg("-c")+" "+c.Args[len(c.Args)-1])
	case "load-buffer":
		if c.Args[len(c.Args)-1] != "-" {
			return local.Output{ExitCode: 1}, nil
		}
		s.buffer = c.Stdin
	case "paste-buffer":
		s.pending, s.buffer = append(s.pending, s.buffer), ""
	case "send-keys":
		if c.Args[len(c.Args)-1] == "Enter" && len(s.pending) > 0 {
			s.typed = append(s.typed, strings.Join(s.pending, ""))
			s.pending = nil
		}
	}
	return local.Output{}, nil
}

func TestTmuxKeepsTheSessionContract(t *testing.T) {
	contracttest.Session(t, func(*testing.T) contracttest.SessionSubject {
		s := &server{}
		return contracttest.SessionSubject{
			Session: tmux.New(s),
			Opened:  func() []string { return s.opened },
			Typed:   func() []string { return s.typed },
			Lines:   func() []string { return s.lines },
		}
	})
}
